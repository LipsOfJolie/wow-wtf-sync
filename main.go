package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type Config struct {
	GamePath    string `json:"game_path"`
	RepoURL     string `json:"repo_url"`
	GitHubToken string `json:"github_token"`
}

const ConfigFileName = "config.json"
const LocalRepoFolder = "./local_repo"
const MaxFolderSizeMB = 150

//go:embed icon.png
var windowIconPNG []byte

func main() {
	text := textForLanguage(systemLanguage())
	myApp := app.NewWithID("wow.sync.launcher")
	windowIcon := fyne.NewStaticResource("icon.png", windowIconPNG)
	myApp.SetIcon(windowIcon)
	myWindow := myApp.NewWindow(text.windowTitle)
	myWindow.SetIcon(windowIcon)
	myWindow.Resize(fyne.NewSize(600, 520))

	config, err := loadConfig()
	if err != nil {
		config = &Config{}
	}

	gamePathEntry := widget.NewEntry()
	gamePathEntry.SetPlaceHolder(text.gamePathPlaceholder)
	gamePathEntry.SetText(config.GamePath)

	repoUrlEntry := widget.NewEntry()
	repoUrlEntry.SetPlaceHolder(text.repoURLPlaceholder)
	repoUrlEntry.SetText(config.RepoURL)

	tokenEntry := widget.NewPasswordEntry()
	tokenEntry.SetPlaceHolder(text.tokenPlaceholder)
	tokenEntry.SetText(config.GitHubToken)

	// Компактный индикатор связи под полями
	connectionLabel := widget.NewLabel(text.connectionChecking)

	// Главный статус-бар для актуальности данных (где новее файлы)
	statusLabel := widget.NewLabel(text.setupInstructions)
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Проверка связи и актуальности данных
	refreshStatusAndConnection := func() {
		gamePath := strings.TrimSpace(gamePathEntry.Text)
		token := strings.TrimSpace(tokenEntry.Text)
		url := strings.TrimSpace(repoUrlEntry.Text)
		if gamePath == "" || token == "" || url == "" {
			fyne.Do(func() {
				connectionLabel.SetText(text.connectionFill)
				statusLabel.SetText(text.setupInstructions)
			})
			return
		}

		go func() {
			isTokenOk := CheckTokenValid(token)
			isRepoOk := CheckRepoValid(url, token)

			fyne.Do(func() {
				if !isTokenOk || !isRepoOk {
					connectionLabel.SetText(text.connectionError)
					statusLabel.SetText(text.syncUnavailable)
					return
				}
				connectionLabel.SetText(text.connectionConnected)
			})

			// Если связь есть, проверяем где данные новее
			if isTokenOk && isRepoOk {
				if _, err := GetRepository(LocalRepoFolder, url, token); err != nil {
					fyne.Do(func() {
						statusLabel.SetText(text.openRepoError + err.Error())
					})
					return
				}
				if err := CopyDir(gamePath, LocalRepoFolder); err != nil {
					fyne.Do(func() {
						statusLabel.SetText(text.checkGameFilesError + err.Error())
					})
					return
				}

				st, err := CheckRepoStatus(LocalRepoFolder, token)
				if err != nil {
					fyne.Do(func() {
						statusLabel.SetText(text.compareError + err.Error())
					})
					return
				}

				fyne.Do(func() {
					switch st {
					case RepoStatusCloudNewer:
						statusLabel.SetText(text.statusCloudNewer)
					case RepoStatusLocalNewer:
						statusLabel.SetText(text.statusLocalNewer)
					case RepoStatusConflict:
						statusLabel.SetText(text.statusConflict)
					case RepoStatusSynced:
						statusLabel.SetText(text.statusSynced)
					}
				})
			}
		}()
	}

	// Авто-проверка при вводе с задержкой
	var checkTimer *time.Timer
	onFieldChanged := func(string) {
		if checkTimer != nil {
			checkTimer.Stop()
		}
		checkTimer = time.AfterFunc(600*time.Millisecond, func() {
			fyne.Do(refreshStatusAndConnection)
		})
	}

	repoUrlEntry.OnChanged = onFieldChanged
	tokenEntry.OnChanged = onFieldChanged

	// Кнопка выбора папки через проводник
	browseBtn := widget.NewButton(text.browseButton, func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				gamePathEntry.SetText(uri.Path())
			}
		}, myWindow)
	})

	var pullBtn *widget.Button
	var pushBtn *widget.Button

	// Кнопка сохранения настроек
	saveConfigBtn := widget.NewButton(text.saveSettings, func() {
		cleanURL := strings.TrimSpace(repoUrlEntry.Text)
		if cleanURL != "" && !strings.HasSuffix(cleanURL, ".git") {
			cleanURL += ".git"
			repoUrlEntry.SetText(cleanURL)
		}

		config.GamePath = gamePathEntry.Text
		config.RepoURL = cleanURL
		config.GitHubToken = tokenEntry.Text

		if err := saveConfig(config); err != nil {
			statusLabel.SetText(text.saveError + err.Error())
			return
		}
		statusLabel.SetText(text.settingsSaved)
		refreshStatusAndConnection()
	})

	// Кнопка Pull (Скачать)
	pullBtn = widget.NewButton(text.pullButton, func() {
		if config.GamePath == "" || config.RepoURL == "" || config.GitHubToken == "" {
			statusLabel.SetText(text.fieldsRequired)
			return
		}

		pullBtn.Disable()
		pushBtn.Disable()
		progressBar.Show()
		progressBar.SetValue(0.1)
		statusLabel.SetText(text.backupCreating)

		go func() {
			defer func() {
				pullBtn.Enable()
				pushBtn.Enable()
				progressBar.Hide()
			}()

			_, err := CreateBackup(config.GamePath)
			if err != nil {
				statusLabel.SetText(text.backupWarning + err.Error())
			}
			progressBar.SetValue(0.3)

			statusLabel.SetText(text.repositoryConnecting)
			_, err = GetRepository(LocalRepoFolder, config.RepoURL, config.GitHubToken)
			if err != nil {
				statusLabel.SetText(text.repositoryError + err.Error())
				return
			}
			progressBar.SetValue(0.5)

			statusLabel.SetText(text.pullDownloading)
			err = GitPull(LocalRepoFolder, config.GitHubToken)
			if err != nil {
				statusLabel.SetText(text.pullError + err.Error())
				return
			}
			progressBar.SetValue(0.8)

			statusLabel.SetText(text.gameUpdating)
			err = CopyDir(LocalRepoFolder, config.GamePath)
			if err != nil {
				statusLabel.SetText(text.gameCopyError + err.Error())
				return
			}

			progressBar.SetValue(1.0)
			time.Sleep(300 * time.Millisecond)
			statusLabel.SetText(text.pullComplete)
		}()
	})

	// Кнопка Push (Загрузить)
	pushBtn = widget.NewButton(text.pushButton, func() {
		if config.GamePath == "" || config.RepoURL == "" || config.GitHubToken == "" {
			statusLabel.SetText(text.fieldsRequired)
			return
		}

		sizeBytes, err := GetFolderSize(config.GamePath)
		if err == nil {
			sizeMB := sizeBytes / (1024 * 1024)
			if sizeMB > MaxFolderSizeMB {
				statusLabel.SetText(fmt.Sprintf(text.folderTooLarge, sizeMB, MaxFolderSizeMB))
				return
			}
		}

		pullBtn.Disable()
		pushBtn.Disable()
		progressBar.Show()
		progressBar.SetValue(0.2)
		statusLabel.SetText(text.folderPreparing)

		go func() {
			defer func() {
				pullBtn.Enable()
				pushBtn.Enable()
				progressBar.Hide()
			}()

			_, err := GetRepository(LocalRepoFolder, config.RepoURL, config.GitHubToken)
			if err != nil {
				statusLabel.SetText(text.repositoryError + err.Error())
				return
			}
			progressBar.SetValue(0.4)

			statusLabel.SetText(text.gameSyncing)
			err = CopyDir(config.GamePath, LocalRepoFolder)
			if err != nil {
				statusLabel.SetText(text.gameReadError + err.Error())
				return
			}
			progressBar.SetValue(0.7)

			statusLabel.SetText(text.pushSending)
			err = GitPush(LocalRepoFolder, config.GitHubToken, "Update SavedVariables from GUI launcher")
			if err != nil {
				statusLabel.SetText(text.pushError + err.Error())
				return
			}

			progressBar.SetValue(1.0)
			time.Sleep(300 * time.Millisecond)
			statusLabel.SetText(text.pushComplete)
		}()
	})

	// Первичная проверка при старте
	refreshStatusAndConnection()

	formContainer := container.NewVBox(
		widget.NewLabel(text.gamePathLabel),
		container.NewBorder(nil, nil, nil, browseBtn, gamePathEntry),
		widget.NewLabel(text.repoURLLabel),
		repoUrlEntry,
		widget.NewLabel(text.tokenLabel),
		tokenEntry,
		connectionLabel, // Компактный индикатор связи
		saveConfigBtn,
		widget.NewSeparator(),
		statusLabel, // Статус актуальности данных (где новее)
		progressBar,
		widget.NewSeparator(),
		container.NewGridWithColumns(2, pullBtn, pushBtn),
	)

	myWindow.SetContent(formContainer)
	myWindow.ShowAndRun()
}

// Получаем путь к папке, где реально лежит запущенный .exe файл
func getConfigPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return ConfigFileName // запасной вариант
	}
	return filepath.Join(filepath.Dir(exePath), ConfigFileName)
}

func loadConfig() (*Config, error) {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); err != nil {
		return nil, err
	}
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	config := &Config{}
	err = decoder.Decode(config)
	return config, err
}

func saveConfig(config *Config) error {
	configPath := getConfigPath()
	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}

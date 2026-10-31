package main

import (
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"fyne.io/systray"
)

type Config struct {
	GamePath       string `json:"game_path"`
	RepoURL        string `json:"repo_url"`
	GitHubToken    string `json:"github_token"`
	StartMinimized bool   `json:"start_minimized"`
	AutoSync       bool   `json:"auto_sync"`
	Autostart      bool   `json:"autostart"`
	LastSyncTime   string `json:"last_sync_time,omitempty"`
	LastSyncStatus string `json:"last_sync_status,omitempty"`
}

const ConfigFileName = "config.json"
const LocalRepoFolder = "local_repo"
const MaxFolderSizeMB = 150
const MaxPushFileSizeMB = 80

//go:embed icon.png
var windowIconPNG []byte

func main() {
	text := textForLanguage(systemLanguage())
	myApp := app.NewWithID("wow.sync.launcher")
	windowIcon := fyne.NewStaticResource("icon.png", windowIconPNG)
	myApp.SetIcon(windowIcon)
	myWindow := myApp.NewWindow("WoWWTFSync")
	myWindow.SetIcon(windowIcon)
	myWindow.Resize(fyne.NewSize(600, 560))

	config, err := loadConfig()
	configLoadError := err
	if err != nil {
		if config == nil {
			config = &Config{}
		}
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "load config:", err)
		}
	}

	var configMu sync.Mutex
	var operationMu sync.Mutex
	getConfig := func() Config {
		configMu.Lock()
		defer configMu.Unlock()
		return *config
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

	connectionLabel := widget.NewLabel(text.connectionChecking)
	statusLabel := widget.NewLabel(text.setupInstructions)
	statusLabel.Wrapping = fyne.TextWrapBreak
	copyStatusButton := widget.NewButton(text.copyStatusButton, func() {
		myWindow.Clipboard().SetContent(statusLabel.Text)
	})
	lastSyncLabel := widget.NewLabel(formatLastSync(config.LastSyncTime, config.LastSyncStatus, text))
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	setStatus := func(status string) {
		fyne.Do(func() { statusLabel.SetText(status) })
	}
	setProgress := func(value float64, visible bool) {
		fyne.Do(func() {
			if visible {
				progressBar.Show()
				progressBar.SetValue(value)
			} else {
				progressBar.Hide()
			}
		})
	}
	recordSync := func(status string) {
		now := time.Now()
		configMu.Lock()
		config.LastSyncTime = now.Format(time.RFC3339)
		config.LastSyncStatus = status
		saveErr := saveConfig(config)
		savedConfig := *config
		configMu.Unlock()

		fyne.Do(func() {
			lastSyncLabel.SetText(formatLastSync(savedConfig.LastSyncTime, savedConfig.LastSyncStatus, text))
			if saveErr != nil {
				statusLabel.SetText(text.saveError + saveErr.Error())
			}
		})
	}

	var pullBtn *widget.Button
	var pushBtn *widget.Button
	var trayStatusItem *fyne.MenuItem
	var trayAutoSyncItem *fyne.MenuItem
	var trayMenu *fyne.Menu
	var autoSyncCheck *widget.Check
	var autoSyncUpdating bool
	var setAutoSyncEnabled func(bool)
	var requestManualSyncChoice func()
	githubReady := make(chan struct{})
	refreshTrayStatus := func() {
		fyne.Do(func() {
			cfg := getConfig()
			trayStatusItem.Label = formatLastSync(cfg.LastSyncTime, cfg.LastSyncStatus, text)
			trayMenu.Refresh()
		})
	}

	persistSyncResult := func(action string, err error) {
		var resultText, notification string
		if err != nil {
			resultText = text.syncFailed + err.Error()
			notification = text.syncFailed + err.Error()
			setStatus(resultText)
		} else {
			switch action {
			case "push":
				resultText = text.lastPushSuccess
				notification = text.lastPushSuccess
			case "pull":
				resultText = text.lastPullSuccess
				notification = text.lastPullSuccess
			default:
				resultText = text.statusSynced
				notification = text.statusSynced
			}
			setStatus(resultText)
		}
		recordSync(resultText)
		refreshTrayStatus()
		myApp.SendNotification(fyne.NewNotification("WoWWTFSync", notification))
	}

	// All repository operations use the same lock because the worktree is shared.
	runPush := func(cfg Config) error {
		if cfg.GamePath == "" || cfg.RepoURL == "" || cfg.GitHubToken == "" {
			return errors.New(text.fieldsRequired)
		}
		maxFileSize := int64(MaxPushFileSizeMB) * 1024 * 1024
		sizeBytes, oversizedFiles, err := savedVariablesFileSizes(cfg.GamePath, maxFileSize)
		if err != nil {
			return fmt.Errorf("%s%w", text.gameReadError, err)
		}
		for _, file := range oversizedFiles {
			log.Printf("WARNING: skipping file larger than %d MB: %q (%d bytes)", MaxPushFileSizeMB, file.Path, file.Size)
		}
		sizeMB := sizeBytes / (1024 * 1024)
		if sizeMB > MaxFolderSizeMB {
			return fmt.Errorf(text.folderTooLarge, sizeMB, MaxFolderSizeMB)
		}
		localRepoPath, err := getLocalRepoPath()
		if err != nil {
			return fmt.Errorf("%s%w", text.repositoryError, err)
		}
		if _, err := GetRepository(localRepoPath, cfg.RepoURL, cfg.GitHubToken); err != nil {
			return fmt.Errorf("%s%w", text.repositoryError, err)
		}
		selectedPaths, err := listSavedVariablesFiles(cfg.GamePath)
		if err != nil {
			return fmt.Errorf("%s%w", text.gameReadError, err)
		}
		if err := copySavedVariablesFiles(cfg.GamePath, localRepoPath, maxFileSize); err != nil {
			return fmt.Errorf("%s%w", text.gameReadError, err)
		}
		if err := removeTrackedFilesMissingFromSource(cfg.GamePath, localRepoPath, maxFileSize); err != nil {
			return fmt.Errorf("%s%w", text.gameReadError, err)
		}
		if err := GitPush(localRepoPath, cfg.GitHubToken, "Synchronize selected folder from WoWWTFSync", selectedPaths); err != nil {
			return fmt.Errorf("%s%w", text.pushError, err)
		}
		return nil
	}

	runPull := func(cfg Config, confirmOverwrite, allowDiverged bool) error {
		if cfg.GamePath == "" || cfg.RepoURL == "" || cfg.GitHubToken == "" {
			return errors.New(text.fieldsRequired)
		}
		if err := requireGameStopped(isGameRunning); err != nil {
			return fmt.Errorf("%s%w", text.pullWhileGameOpen, err)
		}
		localRepoPath, err := getLocalRepoPath()
		if err != nil {
			return fmt.Errorf("%s%w", text.repositoryError, err)
		}
		if _, err := GetRepository(localRepoPath, cfg.RepoURL, cfg.GitHubToken); err != nil {
			return fmt.Errorf("%s%w", text.repositoryError, err)
		}
		if err := ensureCleanWorktree(localRepoPath); err != nil {
			return fmt.Errorf("%s%w", text.localChangesPullBlocked, err)
		}
		repoStatus, err := CheckRepoStatus(localRepoPath, cfg.GitHubToken)
		if err != nil {
			return fmt.Errorf("%s%w", text.compareError, err)
		}
		if !allowDiverged && (repoStatus == RepoStatusConflict || repoStatus == RepoStatusLocalNewer) {
			return errors.New(text.pullConflict)
		}
		gameMatchesRepo, err := savedVariablesDirectoriesHaveSameContents(cfg.GamePath, localRepoPath, int64(MaxPushFileSizeMB)*1024*1024)
		if err != nil {
			return fmt.Errorf("%s%w", text.checkGameFilesError, err)
		}
		if !gameMatchesRepo && !confirmOverwrite {
			return errPullConfirmationRequired
		}
		if err := requireGameStopped(isGameRunning); err != nil {
			return fmt.Errorf("%s%w", text.pullWhileGameOpen, err)
		}
		if _, err := CreateBackup(cfg.GamePath); err != nil {
			return fmt.Errorf("%s%w", text.backupWarning, err)
		}
		previousPaths, err := trackedRepositoryFiles(localRepoPath, int64(MaxPushFileSizeMB)*1024*1024)
		if err != nil {
			return fmt.Errorf("%s%w", text.repositoryError, err)
		}
		if err := GitPull(localRepoPath, cfg.GitHubToken); err != nil {
			return fmt.Errorf("%s%w", text.pullError, err)
		}
		if err := copyTrackedSavedVariables(localRepoPath, cfg.GamePath, int64(MaxPushFileSizeMB)*1024*1024); err != nil {
			return fmt.Errorf("%s%w", text.gameCopyError, err)
		}
		if err := removeTrackedFilesDeletedFromRepository(cfg.GamePath, localRepoPath, previousPaths, int64(MaxPushFileSizeMB)*1024*1024); err != nil {
			return fmt.Errorf("%s%w", text.gameCopyError, err)
		}
		return nil
	}

	runAutomaticSync := func() {
		cfg := getConfig()
		if cfg.GamePath == "" || cfg.RepoURL == "" || cfg.GitHubToken == "" {
			setStatus(text.fieldsRequired)
			return
		}

		operationMu.Lock()
		defer operationMu.Unlock()
		setProgress(0.2, true)
		defer setProgress(0, false)

		localRepoPath, err := getLocalRepoPath()
		if err != nil {
			persistSyncResult("auto", fmt.Errorf("%s%w", text.repositoryError, err))
			return
		}
		_, err = getRepositoryStatusWithRetries(
			localRepoPath,
			cfg.RepoURL,
			cfg.GitHubToken,
			gitStartupAttempts,
			gitStartupRetryInterval,
			waitForRetry,
		)
		if err != nil {
			var repositoryErr *repositoryStatusError
			if errors.As(err, &repositoryErr) && repositoryErr.stage == "repository" {
				persistSyncResult("auto", fmt.Errorf("%s%w", text.repositoryError, err))
				return
			}
			persistSyncResult("auto", fmt.Errorf("%s%w", text.compareError, err))
			return
		}
		action, err := automaticSavedVariablesAction(cfg.GamePath, localRepoPath, int64(MaxPushFileSizeMB)*1024*1024)
		if err != nil {
			persistSyncResult("auto", fmt.Errorf("%s%w", text.checkGameFilesError, err))
			return
		}
		setProgress(0.5, true)

		switch action {
		case autoSyncPush:
			setStatus(text.gameSyncing)
			err = runPush(cfg)
			if err == nil {
				persistSyncResult("push", nil)
			} else {
				persistSyncResult("push", err)
			}
		case autoSyncPull:
			setStatus(text.backupCreating)
			err = runPull(cfg, true, true)
			if err == nil {
				persistSyncResult("pull", nil)
			} else {
				persistSyncResult("pull", err)
			}
		case autoSyncAlreadySynced:
			persistSyncResult("synced", nil)
		case autoSyncConflict:
			setStatus(text.statusConflict)
			fyne.Do(requestManualSyncChoice)
		}
	}

	var refreshStatusAndConnection func()
	refreshStatusAndConnection = func() {
		gamePath := strings.TrimSpace(gamePathEntry.Text)
		token := strings.TrimSpace(tokenEntry.Text)
		if gamePath == "" || token == "" || strings.TrimSpace(repoUrlEntry.Text) == "" {
			connectionLabel.SetText(text.connectionFill)
			statusLabel.SetText(text.setupInstructions)
			return
		}
		url, urlErr := validateGitHubRepoURL(repoUrlEntry.Text)
		if urlErr != nil {
			connectionLabel.SetText(text.connectionError)
			statusLabel.SetText(text.invalidRepoURL)
			return
		}
		select {
		case <-githubReady:
		default:
			connectionLabel.SetText(text.githubWaiting)
			statusLabel.SetText(text.githubWaiting)
			return
		}

		go func() {
			isTokenOK := CheckTokenValid(token)
			isRepoOK := CheckRepoValid(url, token)
			fyne.Do(func() {
				if !isTokenOK || !isRepoOK {
					connectionLabel.SetText(text.connectionError)
					statusLabel.SetText(text.syncUnavailable)
					return
				}
				connectionLabel.SetText(text.connectionConnected)
			})
			if !isTokenOK || !isRepoOK {
				return
			}

			operationMu.Lock()
			defer operationMu.Unlock()
			localRepoPath, err := getLocalRepoPath()
			if err != nil {
				setStatus(text.openRepoError + err.Error())
				return
			}
			st, err := getRepositoryStatusWithRetries(
				localRepoPath,
				url,
				token,
				gitStartupAttempts,
				gitStartupRetryInterval,
				waitForRetry,
			)
			if err != nil {
				var repositoryErr *repositoryStatusError
				if errors.As(err, &repositoryErr) && repositoryErr.stage == "repository" {
					setStatus(text.openRepoError + err.Error())
					return
				}
				setStatus(text.compareError + err.Error())
				return
			}
			switch st {
			case RepoStatusCloudNewer:
				setStatus(text.statusCloudNewer)
			case RepoStatusLocalNewer:
				setStatus(text.statusLocalNewer)
			case RepoStatusConflict:
				setStatus(text.statusConflict)
			case RepoStatusSynced:
				setStatus(text.statusSynced)
			}
		}()
	}

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

	browseBtn := widget.NewButton(text.browseButton, func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				gamePathEntry.SetText(uri.Path())
			}
		}, myWindow)
	})
	startMinimizedCheck := widget.NewCheck(text.startMinimized, func(checked bool) {
		configMu.Lock()
		config.StartMinimized = checked
		saveErr := saveConfig(config)
		configMu.Unlock()
		if saveErr != nil {
			setStatus(text.saveError + saveErr.Error())
		}
	})
	startMinimizedCheck.SetChecked(config.StartMinimized)
	autoSyncCheck = widget.NewCheck(text.autoSync, func(checked bool) {
		if autoSyncUpdating {
			return
		}
		if setAutoSyncEnabled != nil {
			setAutoSyncEnabled(checked)
		}
	})
	autoSyncUpdating = true
	autoSyncCheck.SetChecked(config.AutoSync)
	autoSyncUpdating = false
	autostartUpdating := false
	var autostartCheck *widget.Check
	autostartCheck = widget.NewCheck(text.autostart, func(checked bool) {
		if autostartUpdating {
			return
		}
		if err := setAutostart(checked); err != nil {
			autostartUpdating = true
			autostartCheck.SetChecked(!checked)
			autostartUpdating = false
			setStatus(text.autostartError + err.Error())
			return
		}

		configMu.Lock()
		previous := config.Autostart
		config.Autostart = checked
		saveErr := saveConfig(config)
		if saveErr != nil {
			config.Autostart = previous
		}
		configMu.Unlock()
		if saveErr != nil {
			rollbackErr := setAutostart(previous)
			autostartUpdating = true
			autostartCheck.SetChecked(previous)
			autostartUpdating = false
			if rollbackErr != nil {
				setStatus(text.saveError + saveErr.Error() + "; " + text.autostartError + rollbackErr.Error())
				return
			}
			setStatus(text.saveError + saveErr.Error())
		}
	})
	autostartCheck.SetChecked(config.Autostart)

	saveConfigBtn := widget.NewButton(text.refreshConfiguration, func() {
		cleanURL := strings.TrimSpace(repoUrlEntry.Text)
		if cleanURL != "" {
			normalizedURL, err := validateGitHubRepoURL(cleanURL)
			if err != nil {
				setStatus(text.invalidRepoURL)
				return
			}
			cleanURL = normalizedURL
			repoUrlEntry.SetText(cleanURL)
		}
		configMu.Lock()
		config.GamePath = strings.TrimSpace(gamePathEntry.Text)
		config.RepoURL = cleanURL
		config.GitHubToken = strings.TrimSpace(tokenEntry.Text)
		config.StartMinimized = startMinimizedCheck.Checked
		config.AutoSync = autoSyncCheck.Checked
		config.Autostart = autostartCheck.Checked
		saveErr := saveConfig(config)
		configMu.Unlock()
		if saveErr != nil {
			setStatus(text.saveError + saveErr.Error())
			return
		}
		setStatus(text.settingsSaved)
		refreshStatusAndConnection()
	})

	setOperationBusy := func(busy bool) {
		fyne.Do(func() {
			if busy {
				pullBtn.Disable()
				pushBtn.Disable()
				progressBar.Show()
				progressBar.SetValue(0.1)
			} else {
				pullBtn.Enable()
				pushBtn.Enable()
				progressBar.Hide()
			}
		})
	}
	completeManualOperation := func(action string, err error) {
		persistSyncResult(action, err)
		setOperationBusy(false)
	}
	confirmedPull := func(cfg Config, allowDiverged bool) {
		dialog.ShowConfirm(text.pullConfirmTitle, text.pullConfirmation, func(confirmed bool) {
			if !confirmed {
				setOperationBusy(false)
				return
			}
			go func() {
				operationMu.Lock()
				err := runPull(cfg, true, allowDiverged)
				operationMu.Unlock()
				completeManualOperation("pull", err)
			}()
		}, myWindow)
	}
	var startManualOperation func(action string, allowDiverged bool)
	startManualOperation = func(action string, allowDiverged bool) {
		setOperationBusy(true)
		go func() {
			operationMu.Lock()
			setProgress(0.2, true)
			cfg := getConfig()
			var err error
			if action == "pull" {
				err = runPull(cfg, false, allowDiverged)
			} else {
				err = runPush(cfg)
			}
			operationMu.Unlock()
			if errors.Is(err, errPullConfirmationRequired) {
				fyne.Do(func() { confirmedPull(cfg, allowDiverged) })
				return
			}
			completeManualOperation(action, err)
		}()
	}
	requestManualSyncChoice = func() {
		var choiceDialog *dialog.CustomDialog
		pushChoice := widget.NewButton(text.pushButton, func() {
			choiceDialog.Hide()
			startManualOperation("push", false)
		})
		pullChoice := widget.NewButton(text.pullButton, func() {
			choiceDialog.Hide()
			startManualOperation("pull", true)
		})
		content := container.NewVBox(
			widget.NewLabel(text.syncChoicePrompt),
			container.NewGridWithColumns(2, pushChoice, pullChoice),
		)
		choiceDialog = dialog.NewCustom(text.syncChoiceTitle, text.cancelButton, content, myWindow)
		choiceDialog.Show()
	}
	pullBtn = widget.NewButton(text.pullButton, func() { startManualOperation("pull", false) })
	pushBtn = widget.NewButton(text.pushButton, func() { startManualOperation("push", false) })
	openBackupsButton := widget.NewButton(text.openBackups, func() {
		backupDir, err := getBackupDirectory()
		if err == nil {
			err = openBackupDirectory(backupDir)
		}
		if err != nil {
			setStatus(text.backupOpenError + err.Error())
		}
	})
	restoreBackupButton := widget.NewButton(text.restoreBackup, func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				setStatus(text.backupSelectError + err.Error())
				return
			}
			if reader == nil {
				return
			}
			archivePath := reader.URI().Path()
			if err := reader.Close(); err != nil {
				setStatus(text.backupSelectError + err.Error())
				return
			}
			dialog.ShowConfirm(text.restoreBackup, text.backupRestoreConfirm, func(confirmed bool) {
				if !confirmed {
					return
				}
				go func() {
					operationMu.Lock()
					err := requireGameStopped(isGameRunning)
					if err == nil {
						_, err = RestoreBackup(archivePath, getConfig().GamePath)
					}
					operationMu.Unlock()
					if err != nil {
						setStatus(text.backupWarning + err.Error())
						return
					}
					setStatus(text.backupRestoreSuccess)
				}()
			}, myWindow)
		}, myWindow)
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".zip"}))
		fileDialog.Show()
	})

	showWindowItem := fyne.NewMenuItem(text.showWindow, myWindow.Show)
	pushMenuItem := fyne.NewMenuItem(text.pushButton, func() { startManualOperation("push", false) })
	pullMenuItem := fyne.NewMenuItem(text.pullButton, func() { startManualOperation("pull", false) })
	trayAutoSyncItem = fyne.NewMenuItem(autoSyncMenuLabel(config.AutoSync, text.autoSync), func() {
		setAutoSyncEnabled(!getConfig().AutoSync)
	})
	aboutMenuItem := fyne.NewMenuItem(text.about, func() {
		repositoryURL, err := url.Parse("https://github.com/LipsOfJolie/wow-wtf-sync")
		if err != nil {
			setStatus(text.openRepoError + err.Error())
			return
		}
		if err := myApp.OpenURL(repositoryURL); err != nil {
			setStatus(text.openRepoError + err.Error())
		}
	})
	trayStatusItem = fyne.NewMenuItem(formatLastSync(config.LastSyncTime, config.LastSyncStatus, text), nil)
	trayStatusItem.Disabled = true
	quitMenuItem := fyne.NewMenuItem(text.quit, myApp.Quit)
	quitMenuItem.IsQuit = true
	trayMenu = fyne.NewMenu("WoWWTFSync",
		showWindowItem,
		pushMenuItem,
		pullMenuItem,
		trayAutoSyncItem,
		fyne.NewMenuItemSeparator(),
		trayStatusItem,
		fyne.NewMenuItemSeparator(),
		aboutMenuItem,
		fyne.NewMenuItemSeparator(),
		quitMenuItem,
	)
	setAutoSyncEnabled = func(enabled bool) {
		configMu.Lock()
		previous := config.AutoSync
		config.AutoSync = enabled
		saveErr := saveConfig(config)
		if saveErr != nil {
			config.AutoSync = previous
		}
		actual := config.AutoSync
		configMu.Unlock()

		fyne.Do(func() {
			autoSyncUpdating = true
			autoSyncCheck.SetChecked(actual)
			autoSyncUpdating = false
			trayAutoSyncItem.Label = autoSyncMenuLabel(actual, text.autoSync)
			trayMenu.Refresh()
		})
		if saveErr != nil {
			setStatus(text.saveError + saveErr.Error())
		}
	}
	if desktopApp, ok := myApp.(desktop.App); ok {
		desktopApp.SetSystemTrayMenu(trayMenu)
		desktopApp.SetSystemTrayWindow(myWindow)
	}
	var trayClickMu sync.Mutex
	var lastTrayClick time.Time
	systray.SetOnTapped(func() {
		trayClickMu.Lock()
		now := time.Now()
		doubleClick := !lastTrayClick.IsZero() && now.Sub(lastTrayClick) <= 500*time.Millisecond
		if doubleClick {
			lastTrayClick = time.Time{}
		} else {
			lastTrayClick = now
		}
		trayClickMu.Unlock()
		if doubleClick {
			fyne.Do(myWindow.Show)
		}
	})
	myWindow.SetCloseIntercept(myWindow.Hide)

	formContainer := container.NewVBox(
		widget.NewLabel(text.gamePathLabel),
		container.NewBorder(nil, nil, nil, browseBtn, gamePathEntry),
		widget.NewLabel(text.repoURLLabel),
		repoUrlEntry,
		widget.NewLabel(text.tokenLabel),
		tokenEntry,
		connectionLabel,
		container.NewVBox(startMinimizedCheck, autoSyncCheck, autostartCheck),
		saveConfigBtn,
		widget.NewSeparator(),
		statusLabel,
		copyStatusButton,
		progressBar,
		widget.NewSeparator(),
		container.NewGridWithColumns(2, pullBtn, pushBtn),
		container.NewGridWithColumns(2, openBackupsButton, restoreBackupButton),
		widget.NewSeparator(),
		lastSyncLabel,
	)
	myWindow.SetContent(formContainer)

	refreshStatusAndConnection()
	if configLoadError != nil && !errors.Is(configLoadError, os.ErrNotExist) {
		setStatus(text.loadConfigError + configLoadError.Error())
	}
	go monitorGitHubAvailability(
		getConfig,
		githubReady,
		func() { fyne.Do(refreshStatusAndConnection) },
		func() {
			fyne.Do(func() {
				connectionLabel.SetText(text.githubWaiting)
				statusLabel.SetText(text.githubWaiting)
			})
		},
		isGitHubReachable,
	)
	go monitorAutoSyncWithProbe(getConfig, runAutomaticSync, setStatus, text, isGameRunning, config.AutoSync, githubReady)
	if config.StartMinimized {
		myApp.Run()
	} else {
		myWindow.ShowAndRun()
	}
}

func autoSyncMenuLabel(enabled bool, label string) string {
	if enabled {
		return "☑ " + label
	}
	return "☐ " + label
}

func formatLastSync(timestamp, status string, text uiText) string {
	if timestamp == "" || status == "" {
		return text.lastSyncNever
	}
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Sprintf("%s. %s%s", status, text.lastSyncLabel, timestamp)
	}
	return fmt.Sprintf("%s. %s%s", status, text.lastSyncLabel, parsed.Local().Format("02.01.2006 15:04"))
}

func getLocalRepoPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить папку данных пользователя: %w", err)
	}

	localRepoPath := filepath.Join(cacheDir, "WoWWTFSync", LocalRepoFolder)
	if err := rejectUnsafePathComponents(filepath.Dir(localRepoPath)); err != nil {
		return "", fmt.Errorf("unsafe application cache directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(localRepoPath), 0700); err != nil {
		return "", fmt.Errorf("не удалось создать папку данных приложения %q: %w", filepath.Dir(localRepoPath), err)
	}
	return localRepoPath, nil
}

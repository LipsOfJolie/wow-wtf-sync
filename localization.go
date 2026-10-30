package main

import "strings"

type uiText struct {
	windowTitle          string
	gamePathLabel        string
	gamePathPlaceholder  string
	browseButton         string
	repoURLLabel         string
	repoURLPlaceholder   string
	tokenLabel           string
	tokenPlaceholder     string
	connectionChecking   string
	connectionConnected  string
	connectionFill       string
	setupInstructions    string
	connectionError      string
	syncUnavailable      string
	openRepoError        string
	checkGameFilesError  string
	compareError         string
	statusCloudNewer     string
	statusLocalNewer     string
	statusConflict       string
	statusSynced         string
	saveSettings         string
	saveError            string
	settingsSaved        string
	pullButton           string
	pushButton           string
	fieldsRequired       string
	backupCreating       string
	backupWarning        string
	repositoryConnecting string
	repositoryError      string
	pullDownloading      string
	pullError            string
	gameUpdating         string
	gameCopyError        string
	pullComplete         string
	folderPreparing      string
	folderTooLarge       string
	gameSyncing          string
	gameReadError        string
	pushSending          string
	pushError            string
	pushComplete         string
}

func textForLanguage(language string) uiText {
	if language == "ru" {
		return uiText{
			windowTitle:          "WoW SavedVariables Launcher",
			gamePathLabel:        "Путь к папке Account в игре:",
			gamePathPlaceholder:  `Например: C:\Program Files (x86)\World of Warcraft\_retail_\WTF\Account`,
			browseButton:         "Обзор...",
			repoURLLabel:         "Ссылка на GitHub репозиторий:",
			repoURLPlaceholder:   "Например: https://github.com/username/repository.git",
			tokenLabel:           "GitHub Personal Access Token:",
			tokenPlaceholder:     "Вставьте GitHub Personal Access Token",
			connectionChecking:   "🔌 GitHub: проверка подключения...",
			connectionConnected:  "🔌 Связь с GitHub: ✅ Подключено",
			connectionFill:       "🔌 GitHub: заполните настройки",
			setupInstructions:    "Укажите папку Account, ссылку на репозиторий и токен, затем сохраните настройки.",
			connectionError:      "🔌 Связь с GitHub: ❌ Ошибка (проверьте токен и ссылку)",
			syncUnavailable:      "⚠️ Синхронизация недоступна из-за ошибок подключения.",
			openRepoError:        "⚠️ Не удалось открыть репозиторий: ",
			checkGameFilesError:  "⚠️ Не удалось проверить файлы игры: ",
			compareError:         "⚠️ Не удалось сравнить данные: ",
			statusCloudNewer:     "☁️ В облаке данные новее. Рекомендуется скачать.",
			statusLocalNewer:     "⬆️ Данные в облаке устарели. Рекомендуется загрузить.",
			statusConflict:       "⚠️ Изменения есть и на ПК, и в облаке. Проверьте версии вручную.",
			statusSynced:         "✅ Данные в облаке совпадают с данными на ПК. Действий не требуется.",
			saveSettings:         "Сохранить настройки",
			saveError:            "❌ Ошибка сохранения: ",
			settingsSaved:        "✅ Настройки сохранены!",
			pullButton:           "Скачать с GitHub (Pull)",
			pushButton:           "Загрузить на GitHub (Push)",
			fieldsRequired:       "❌ Ошибка: заполните все поля конфигурации!",
			backupCreating:       "⏳ Создаем локальный бэкап...",
			backupWarning:        "⚠️ Предупреждение бэкапа: ",
			repositoryConnecting: "⏳ Подключение к GitHub репозиторию...",
			repositoryError:      "❌ Ошибка репозитория: ",
			pullDownloading:      "⏳ Скачивание изменений с GitHub...",
			pullError:            "❌ Ошибка Pull: ",
			gameUpdating:         "⏳ Обновление файлов в папке игры...",
			gameCopyError:        "❌ Ошибка копирования в игру: ",
			pullComplete:         "👉 Игра обновлена с GitHub (файлы актуальны).",
			folderPreparing:      "⏳ Проверка папки и подготовка...",
			folderTooLarge:       "❌ Ошибка защиты: папка весит %d МБ (лимит %d МБ). Выбрана неверная папка!",
			gameSyncing:          "⏳ Синхронизация файлов игры с репозиторием...",
			gameReadError:        "❌ Ошибка чтения папки игры: ",
			pushSending:          "⏳ Отправка изменений на GitHub...",
			pushError:            "❌ Ошибка Push (возможно, нет изменений): ",
			pushComplete:         "👉 Изменения успешно отправлены в облако.",
		}
	}

	return uiText{
		windowTitle:          "WoW SavedVariables Launcher",
		gamePathLabel:        "Path to the game's Account folder:",
		gamePathPlaceholder:  `Example: C:\Program Files (x86)\World of Warcraft\_retail_\WTF\Account`,
		browseButton:         "Browse...",
		repoURLLabel:         "GitHub repository URL:",
		repoURLPlaceholder:   "Example: https://github.com/username/repository.git",
		tokenLabel:           "GitHub Personal Access Token:",
		tokenPlaceholder:     "Enter your GitHub Personal Access Token",
		connectionChecking:   "🔌 GitHub: checking connection...",
		connectionConnected:  "🔌 GitHub: ✅ Connected",
		connectionFill:       "🔌 GitHub: enter settings",
		setupInstructions:    "Enter the Account folder, repository URL, and token, then save your settings.",
		connectionError:      "🔌 GitHub connection: ❌ Error (check the token and URL)",
		syncUnavailable:      "⚠️ Sync is unavailable because the connection failed.",
		openRepoError:        "⚠️ Could not open repository: ",
		checkGameFilesError:  "⚠️ Could not check game files: ",
		compareError:         "⚠️ Could not compare data: ",
		statusCloudNewer:     "☁️ Cloud data is newer. Download is recommended.",
		statusLocalNewer:     "⬆️ Cloud data is outdated. Upload is recommended.",
		statusConflict:       "⚠️ Changes exist both locally and in the cloud. Review them manually.",
		statusSynced:         "✅ Cloud data matches this PC. No action is needed.",
		saveSettings:         "Save settings",
		saveError:            "❌ Could not save settings: ",
		settingsSaved:        "✅ Settings saved!",
		pullButton:           "Download from GitHub (Pull)",
		pushButton:           "Upload to GitHub (Push)",
		fieldsRequired:       "❌ Error: fill in all configuration fields.",
		backupCreating:       "⏳ Creating a local backup...",
		backupWarning:        "⚠️ Backup warning: ",
		repositoryConnecting: "⏳ Connecting to the GitHub repository...",
		repositoryError:      "❌ Repository error: ",
		pullDownloading:      "⏳ Downloading changes from GitHub...",
		pullError:            "❌ Pull error: ",
		gameUpdating:         "⏳ Updating files in the game folder...",
		gameCopyError:        "❌ Could not copy files to the game: ",
		pullComplete:         "👉 Game files were updated from GitHub.",
		folderPreparing:      "⏳ Checking the folder and preparing...",
		folderTooLarge:       "❌ Safety check: folder is %d MB (limit %d MB). The selected folder may be incorrect.",
		gameSyncing:          "⏳ Syncing game files with the repository...",
		gameReadError:        "❌ Could not read the game folder: ",
		pushSending:          "⏳ Uploading changes to GitHub...",
		pushError:            "❌ Push error (there may be no changes): ",
		pushComplete:         "👉 Changes were successfully uploaded to the cloud.",
	}
}

func isRussianLocale(language string) bool {
	language = strings.ToLower(strings.TrimSpace(language))
	return language == "ru" || strings.HasPrefix(language, "ru-")
}

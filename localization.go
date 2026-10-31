package main

import "strings"

type uiText struct {
	windowTitle             string
	gamePathLabel           string
	gamePathPlaceholder     string
	browseButton            string
	repoURLLabel            string
	repoURLPlaceholder      string
	tokenLabel              string
	tokenPlaceholder        string
	connectionChecking      string
	connectionConnected     string
	githubWaiting           string
	connectionFill          string
	setupInstructions       string
	connectionError         string
	syncUnavailable         string
	openRepoError           string
	checkGameFilesError     string
	compareError            string
	statusCloudNewer        string
	statusLocalNewer        string
	statusConflict          string
	statusSynced            string
	refreshConfiguration    string
	saveError               string
	loadConfigError         string
	settingsSaved           string
	pullButton              string
	pushButton              string
	fieldsRequired          string
	backupCreating          string
	backupWarning           string
	repositoryConnecting    string
	repositoryError         string
	pullDownloading         string
	pullError               string
	gameUpdating            string
	gameCopyError           string
	pullComplete            string
	folderPreparing         string
	folderTooLarge          string
	gameSyncing             string
	gameReadError           string
	pushSending             string
	pushError               string
	pushComplete            string
	startMinimized          string
	autoSync                string
	showWindow              string
	about                   string
	quit                    string
	lastSyncLabel           string
	lastSyncNever           string
	processCheckError       string
	syncFailed              string
	lastPushSuccess         string
	lastPullSuccess         string
	autostart               string
	autostartError          string
	autoSyncChecking        string
	copyStatusButton        string
	invalidRepoURL          string
	pullWhileGameOpen       string
	localChangesPullBlocked string
	pullConflict            string
	syncChoiceTitle         string
	syncChoicePrompt        string
	cancelButton            string
	pullConfirmTitle        string
	pullConfirmation        string
	openBackups             string
	restoreBackup           string
	backupOpenError         string
	backupSelectError       string
	backupRestoreConfirm    string
	backupRestoreSuccess    string
}

func textForLanguage(language string) uiText {
	if language == "ru" {
		return uiText{
			windowTitle:             "WoWWTFSync",
			gamePathLabel:           "Путь к папке Account в игре:",
			gamePathPlaceholder:     `Например: C:\Program Files (x86)\World of Warcraft\_retail_\WTF\Account`,
			browseButton:            "Обзор...",
			repoURLLabel:            "Ссылка на GitHub репозиторий:",
			repoURLPlaceholder:      "Например: https://github.com/username/repository.git",
			tokenLabel:              "GitHub Personal Access Token:",
			tokenPlaceholder:        "Вставьте GitHub Personal Access Token",
			connectionChecking:      "🔌 GitHub: проверка подключения...",
			connectionConnected:     "🔌 Связь с GitHub: ✅ Подключено",
			githubWaiting:           "🔌 Ожидание подключения к GitHub...",
			connectionFill:          "🔌 GitHub: заполните настройки",
			setupInstructions:       "Укажите папку Account, ссылку на репозиторий и токен, затем сохраните настройки.",
			connectionError:         "🔌 Связь с GitHub: ❌ Ошибка (проверьте токен и ссылку)",
			syncUnavailable:         "⚠️ Синхронизация недоступна из-за ошибок подключения.",
			openRepoError:           "⚠️ Не удалось открыть репозиторий: ",
			checkGameFilesError:     "⚠️ Не удалось проверить файлы игры: ",
			compareError:            "⚠️ Не удалось сравнить данные: ",
			statusCloudNewer:        "☁️ В облаке данные новее. Рекомендуется скачать.",
			statusLocalNewer:        "⬆️ Данные в облаке устарели. Рекомендуется загрузить.",
			statusConflict:          "⚠️ Не удалось однозначно определить более новые файлы. Выберите Push или Pull.",
			statusSynced:            "Синхронизация не требуется",
			refreshConfiguration:    "Обновить конфигурацию / Refresh",
			saveError:               "❌ Ошибка сохранения: ",
			loadConfigError:         "❌ Ошибка загрузки конфигурации: ",
			settingsSaved:           "✅ Настройки сохранены!",
			pullButton:              "Скачать с GitHub (Pull)",
			pushButton:              "Загрузить на GitHub (Push)",
			fieldsRequired:          "❌ Ошибка: заполните все поля конфигурации!",
			backupCreating:          "⏳ Создаем локальный бэкап...",
			backupWarning:           "⚠️ Предупреждение бэкапа: ",
			repositoryConnecting:    "⏳ Подключение к GitHub репозиторию...",
			repositoryError:         "❌ Ошибка репозитория: ",
			pullDownloading:         "⏳ Скачивание изменений с GitHub...",
			pullError:               "❌ Ошибка Pull: ",
			gameUpdating:            "⏳ Обновление файлов в папке игры...",
			gameCopyError:           "❌ Ошибка копирования в игру: ",
			pullComplete:            "👉 Игра обновлена с GitHub (файлы актуальны).",
			folderPreparing:         "⏳ Проверка папки и подготовка...",
			folderTooLarge:          "❌ Ошибка защиты: папка весит %d МБ (лимит %d МБ). Выбрана неверная папка!",
			gameSyncing:             "⏳ Синхронизация файлов игры с репозиторием...",
			gameReadError:           "❌ Ошибка чтения папки игры: ",
			pushSending:             "⏳ Отправка изменений на GitHub...",
			pushError:               "❌ Ошибка Push (возможно, нет изменений): ",
			pushComplete:            "👉 Изменения успешно отправлены в облако.",
			startMinimized:          "Запускать свернутым в трей",
			autoSync:                "Автоматическая синхронизация",
			showWindow:              "Показать окно",
			about:                   "О программе",
			quit:                    "Выход",
			lastSyncLabel:           "Последняя синхронизация: ",
			lastSyncNever:           "Последняя синхронизация: еще не выполнялась",
			processCheckError:       "❌ Не удалось проверить процесс игры: ",
			syncFailed:              "❌ Ошибка синхронизации: ",
			lastPushSuccess:         "Успешный Push",
			lastPullSuccess:         "Успешный Pull",
			autostart:               "Запускать приложение при входе в Windows",
			autostartError:          "❌ Не удалось изменить автозапуск: ",
			autoSyncChecking:        "Проверка данных для автосинхронизации...",
			copyStatusButton:        "Копировать сообщение",
			invalidRepoURL:          "❌ Разрешены только HTTPS-ссылки на github.com с адресом вида /владелец/репозиторий.",
			pullWhileGameOpen:       "❌ Закройте wow.exe перед скачиванием данных.",
			localChangesPullBlocked: "❌ Pull остановлен: в локальной Git-копии есть незакоммиченные изменения. Сначала сохраните или разрешите их вручную. ",
			pullConflict:            "⚠️ Ветки разошлись; данные не изменены. Безопасно объедините версии в GitHub Desktop, затем повторите Pull.",
			syncChoiceTitle:         "Выберите направление синхронизации",
			syncChoicePrompt:        "В разных файлах обнаружены более новые версии с обеих сторон. Сравните версии и выберите, куда отправить данные.",
			cancelButton:            "Отмена",
			pullConfirmTitle:        "Заменить файлы выбранной папки?",
			pullConfirmation:        "Содержимое выбранной папки отличается от GitHub. Перед Pull будет создан резервный архив всей папки; отсутствующие в GitHub отслеживаемые файлы будут удалены. Продолжить?",
			openBackups:             "Открыть папку резервных копий",
			restoreBackup:           "Восстановить резервную копию...",
			backupOpenError:         "❌ Не удалось открыть папку резервных копий: ",
			backupSelectError:       "❌ Не удалось выбрать резервную копию: ",
			backupRestoreConfirm:    "Перед восстановлением будет создан резервный архив текущей выбранной папки. Продолжить восстановление выбранного архива?",
			backupRestoreSuccess:    "✅ Резервная копия восстановлена.",
		}
	}

	return uiText{
		windowTitle:             "WoWWTFSync",
		gamePathLabel:           "Path to the game's Account folder:",
		gamePathPlaceholder:     `Example: C:\Program Files (x86)\World of Warcraft\_retail_\WTF\Account`,
		browseButton:            "Browse...",
		repoURLLabel:            "GitHub repository URL:",
		repoURLPlaceholder:      "Example: https://github.com/username/repository.git",
		tokenLabel:              "GitHub Personal Access Token:",
		tokenPlaceholder:        "Enter your GitHub Personal Access Token",
		connectionChecking:      "🔌 GitHub: checking connection...",
		connectionConnected:     "🔌 GitHub: ✅ Connected",
		githubWaiting:           "🔌 Waiting for a connection to GitHub...",
		connectionFill:          "🔌 GitHub: enter settings",
		setupInstructions:       "Enter the Account folder, repository URL, and token, then save your settings.",
		connectionError:         "🔌 GitHub connection: ❌ Error (check the token and URL)",
		syncUnavailable:         "⚠️ Sync is unavailable because the connection failed.",
		openRepoError:           "⚠️ Could not open repository: ",
		checkGameFilesError:     "⚠️ Could not check game files: ",
		compareError:            "⚠️ Could not compare data: ",
		statusCloudNewer:        "☁️ Cloud data is newer. Download is recommended.",
		statusLocalNewer:        "⬆️ Cloud data is outdated. Upload is recommended.",
		statusConflict:          "⚠️ Could not determine unambiguously which files are newer. Choose Push or Pull.",
		statusSynced:            "No synchronization is needed",
		refreshConfiguration:    "Refresh configuration",
		saveError:               "❌ Could not save settings: ",
		loadConfigError:         "❌ Could not load configuration: ",
		settingsSaved:           "✅ Settings saved!",
		pullButton:              "Download from GitHub (Pull)",
		pushButton:              "Upload to GitHub (Push)",
		fieldsRequired:          "❌ Error: fill in all configuration fields.",
		backupCreating:          "⏳ Creating a local backup...",
		backupWarning:           "⚠️ Backup warning: ",
		repositoryConnecting:    "⏳ Connecting to the GitHub repository...",
		repositoryError:         "❌ Repository error: ",
		pullDownloading:         "⏳ Downloading changes from GitHub...",
		pullError:               "❌ Pull error: ",
		gameUpdating:            "⏳ Updating files in the game folder...",
		gameCopyError:           "❌ Could not copy files to the game: ",
		pullComplete:            "👉 Game files were updated from GitHub.",
		folderPreparing:         "⏳ Checking the folder and preparing...",
		folderTooLarge:          "❌ Safety check: folder is %d MB (limit %d MB). The selected folder may be incorrect.",
		gameSyncing:             "⏳ Syncing game files with the repository...",
		gameReadError:           "❌ Could not read the game folder: ",
		pushSending:             "⏳ Uploading changes to GitHub...",
		pushError:               "❌ Push error (there may be no changes): ",
		pushComplete:            "👉 Changes were successfully uploaded to the cloud.",
		startMinimized:          "Start minimized to the system tray",
		autoSync:                "Automatic synchronization",
		showWindow:              "Show window",
		about:                   "About",
		quit:                    "Quit",
		lastSyncLabel:           "Last synchronization: ",
		lastSyncNever:           "Last synchronization: not yet performed",
		processCheckError:       "❌ Could not check the game process: ",
		syncFailed:              "❌ Synchronization failed: ",
		lastPushSuccess:         "Successful Push",
		lastPullSuccess:         "Successful Pull",
		autostart:               "Start application when signing in to Windows",
		autostartError:          "❌ Could not update Windows startup: ",
		autoSyncChecking:        "Checking data for automatic synchronization...",
		copyStatusButton:        "Copy message",
		invalidRepoURL:          "❌ Only HTTPS GitHub repository URLs in /owner/repository format are allowed.",
		pullWhileGameOpen:       "❌ Close wow.exe before pulling data.",
		localChangesPullBlocked: "❌ Pull stopped: the local Git working copy has uncommitted changes. Save or resolve them manually first. ",
		pullConflict:            "⚠️ The branches have diverged; no data was changed. Safely reconcile the versions in GitHub Desktop, then retry Pull.",
		syncChoiceTitle:         "Choose synchronization direction",
		syncChoicePrompt:        "Different files have newer versions on both sides. Compare the versions and choose where to synchronize.",
		cancelButton:            "Cancel",
		pullConfirmTitle:        "Replace files in the selected folder?",
		pullConfirmation:        "The selected folder differs from GitHub. A backup of the entire folder will be created before Pull; tracked files removed from GitHub will be deleted locally. Continue?",
		openBackups:             "Open backup folder",
		restoreBackup:           "Restore a backup...",
		backupOpenError:         "❌ Could not open the backup folder: ",
		backupSelectError:       "❌ Could not select a backup: ",
		backupRestoreConfirm:    "A backup of the current selected folder will be created before restoring. Continue with the selected archive?",
		backupRestoreSuccess:    "✅ Backup restored.",
	}
}

func isRussianLocale(language string) bool {
	language = strings.ToLower(strings.TrimSpace(language))
	return language == "ru" || strings.HasPrefix(language, "ru-")
}

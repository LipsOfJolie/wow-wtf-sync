# WoW WTF Sync

Лаунчер для синхронизации выбранной папки World of Warcraft (например, `WTF/Account`) между несколькими ПК с помощью GitHub-репозитория.

## 🚀 Быстрый старт

1. **Создайте репозиторий на GitHub:**
   - Пустой, **Private** (приватный), без `README` и `.gitignore`.
2. **Создайте GitHub Token (PAT):**
   - В настройках профиля (Developer Settings -> Personal Access Tokens).
   - Дайте права **`repo`** (или `Contents: Read and write` для конкретного репозитория).
3. **Заполни данные в приложении**


⚙️ Использование
Запустите `WoWWTFSync`. Конфигурация хранится в `%AppData%\WoWWTFSync\config.json`; при первом запуске старый `config.json` рядом с `.exe` переносится автоматически.

Укажите путь к папке аккаунта игры (.../WTF/Account), ссылку на репозиторий и токен.

Нажмите «Сохранить настройки».

Используйте Push для выгрузки настроек в облако и Pull для скачивания на другой ПК.

Включите «Автоматическая синхронизация», чтобы сравнивать SHA-256 всех файлов выбранной папки и GitHub. Проверка выполняется при запуске программы и через 3 секунды после подтверждённого завершения всех процессов игры; во время работы игры фоновых проверок нет. При совпадении хэшей синхронизация не требуется. Новые, изменённые и удалённые файлы учитываются. Для изменённых с обеих сторон файлов более старая версия заменяется новой по времени локального файла и последнего Git-коммита; при равном времени приоритет у версии на ПК. Удаления определяются относительно последней общей версии. Если разные файлы новее на разных сторонах, программа попросит выбрать Push или Pull. Перед автоматическим Pull создается резервная копия всей папки.

Флажок «Запускать свернутым в трей» запускает приложение в фоне. Закрытие окна скрывает его; команды показа окна, Push, Pull, открытия репозитория на GitHub и выхода доступны из меню трея. Если при запуске нет доступа к GitHub, приложение дождется подключения перед первой автоматической проверкой синхронизации.

Флажок «Запускать приложение при входе в Windows» включает автозапуск для текущего пользователя без прав администратора.

Двойной щелчок левой кнопкой по значку в трее открывает окно, щелчок правой кнопкой открывает контекстное меню. Автоматическую синхронизацию можно включить или выключить как в окне приложения, так и в контекстном меню.

Дата и результат последней синхронизации отображаются в окне и меню трея и сохраняются в конфигурации в `%AppData%\WoWWTFSync`.

Локальная рабочая копия Git-репозитория хранится в пользовательской папке кэша, чтобы запуск из автозагрузки не зависел от текущей папки Windows. Текст текущего сообщения, в том числе ошибки, можно скопировать кнопкой «Копировать сообщение».

Push и Pull синхронизируют всё содержимое выбранной папки `Account` и её вложенных каталогов. Вы можете выбрать и другую папку. Добавления и изменения отправляются в GitHub; удаление файла также передаётся на другую сторону. При Pull файлы, удалённые из GitHub, удаляются локально, но только если они отслеживались в Git; перед Pull создаётся резервная ZIP-копия выбранной папки. Символические ссылки не поддерживаются, а файлы больше 80 МБ пропускаются.

Резервные копии создаются в папке `backups` рядом с `.exe`, независимо от текущей папки запуска; сохраняются только 5 последних архивов. Архив содержит всё содержимое выбранной папки. Кнопка открытия папки показывает архивы, а восстановление предварительно создаёт резервную копию текущих данных. Если Windows запрещает запись в папку, приложение покажет путь и причину ошибки; атрибут «только чтение» автоматически не изменяется.

При Push файлы больше 80 МБ пропускаются и записываются в журнал предупреждений; остальные файлы синхронизируются как обычно. При начальной проверке GitHub выполняется до 3 попыток с интервалом 5 секунд. Если сеть недоступна, приложение продолжает проверять подключение в следующих циклах; сетевые Git-операции ограничены таймаутом в 60 секунд.

URL репозитория принимается только для `https://github.com/<владелец>/<репозиторий>`. Токен защищён Windows DPAPI и не записывается в конфигурацию открытым текстом. При смене URL прежняя локальная копия сохраняется под резервным именем, а её origin сверяется с настройкой.

--------------------------------------------------------------
# WoW WTF Sync

A lightweight launcher for synchronizing a selected World of Warcraft folder (such as `WTF/Account`) across multiple PCs with a GitHub repository.

## 🚀 Quick Start

1. **Create a GitHub Repository:**
   - Empty, **Private**, without `README` or `.gitignore`.
2. **Generate a GitHub Token (PAT):**
   - Go to Profile Settings -> Developer Settings -> Personal Access Tokens.
   - Grant **`repo`** permissions (or `Contents: Read and write` for the specific repository).
3. **Fill in your details in the app**

## ⚙️ Usage

1. Run `WoWWTFSync`. Configuration is stored in `%AppData%\WoWWTFSync\config.json`; an existing legacy config next to the executable is migrated automatically.
2. Specify the path to your game account folder (`.../WTF/Account`), your repository URL, and your token.
3. Click **"Save settings"**.
4. Use **Push** to upload your settings to the cloud and **Pull** to download them on another PC.

Enable **Automatic synchronization** to compare SHA-256 hashes of every file in the selected folder with GitHub. It runs at application startup and three seconds after all `wow.exe` processes are confirmed closed; there are no background checks while the game is running. Matching hashes require no sync. New, changed, and deleted files are included. For files changed on both sides, the older version is replaced using the local file modification time and the last Git commit time; ties prefer the PC copy. Deletions are detected relative to the last common version. If different files are newer on different sides, the app asks the user to choose Push or Pull. An automatic Pull first backs up the entire folder.

Enable **Start minimized to the system tray** to run in the background. Closing the window hides it; the tray menu provides show, Push, Pull, opening the GitHub repository, and quit actions. If GitHub is unreachable at startup, the app waits for a connection before its first automatic synchronization check.

Enable **Start application when signing in to Windows** to add/remove the current user startup entry (no administrator permission required).

Double-click the tray icon with the left mouse button to open the window, or right-click it to open the context menu. Automatic synchronization can be toggled either in the app window or in the context menu.

The last synchronization result and its date/time are shown in the window and tray menu and saved in the configuration under `%AppData%\WoWWTFSync`.

The local Git working copy is stored in the user's cache directory so startup does not depend on Windows' current working directory. Use **Copy message** to copy the current status text, including errors.

Push and Pull synchronize all contents of the selected `Account` folder and its subdirectories; any other folder can also be selected. Additions and changes are pushed to GitHub, and file deletions are propagated to the other side. During Pull, files removed from GitHub are deleted locally only if they were Git-tracked; a ZIP backup of the selected folder is created first. Symbolic links are not supported, and files over 80 MB are skipped.

Backups are saved in a `backups` folder next to the executable; only the 5 most recent archives are kept. Each archive contains the full selected folder. Restore first backs up current data. The app can open the backup folder. If Windows denies write access, it reports the path and error; it does not automatically change the folder's read-only attribute.

During Push, files larger than 80 MB are skipped and logged as warnings. The initial GitHub check makes up to 3 attempts, 5 seconds apart. If the network is unavailable, the app keeps checking in later cycles; Git network operations have a 60-second timeout.

Repository URLs are restricted to `https://github.com/<owner>/<repository>`. The token is protected with Windows DPAPI and is not stored in plain text. The local repository origin is checked against the configured URL; a previous working copy is preserved under a backup name when the repository URL changes.

# WoW WTF Sync

Лаунчер для синхронизации папок `SavedVariables` World of Warcraft с GitHub-репозиторием.

## 🚀 Быстрый старт

1. **Создайте репозиторий на GitHub:**
   - Пустой, **Private** (приватный), без `README` и `.gitignore`.
2. **Создайте GitHub Token (PAT):**
   - В настройках профиля (Developer Settings -> Personal Access Tokens).
   - Дайте права **`repo`** (или `Contents: Read and write` для конкретного репозитория).
3. **Заполни данные в приложении**


⚙️ Использование
Запустите wow-sync-launcher.exe (рядом с ним автоматически создастся файл config.json).

Укажите путь к папке аккаунта игры (.../WTF/Account), ссылку на репозиторий и токен.

Нажмите «Сохранить настройки».

Используйте Push для выгрузки настроек в облако и Pull для скачивания на другой ПК.

⚠️ Предупреждение о безопасности: Файл config.json хранит токен в открытом виде. Держите его в секрете и никогда не загружайте в публичные репозитории.

--------------------------------------------------------------
# WoW WTF Sync

A lightweight launcher for synchronizing World of Warcraft `SavedVariables` folders with a GitHub repository.

## 🚀 Quick Start

1. **Create a GitHub Repository:**
   - Empty, **Private**, without `README` or `.gitignore`.
2. **Generate a GitHub Token (PAT):**
   - Go to Profile Settings -> Developer Settings -> Personal Access Tokens.
   - Grant **`repo`** permissions (or `Contents: Read and write` for the specific repository).
3. **Fill in your details in the app**

## ⚙️ Usage

1. Run `wow-sync-launcher.exe` (a `config.json` file will be created automatically next to it).
2. Specify the path to your game account folder (`.../WTF/Account`), your repository URL, and your token.
3. Click **"Save settings"**.
4. Use **Push** to upload your settings to the cloud and **Pull** to download them on another PC.

> **⚠️ Security Warning:** The `config.json` file stores your token in plain text. Keep it private and never commit it to public repositories.

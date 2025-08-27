# ReleaseNoJutsu 🥷

ReleaseNoJutsu is a personal manga update assistant. It tracks your favourite manga series on MangaDex and provides notifications for new chapters via Telegram. It also allows you to manage your reading progress conveniently through a Telegram bot interface.

## Features ✨

- **📖 Track Manga:** Add manga by providing its MangaDex ID.
- **🗑️ Remove Manga:** Stop tracking a manga you no longer wish to follow.
- **🔔 Notifications:** Receive updates about new chapters every 6 hours.
- **✅ Progress Management:** Mark chapters as read or unread.
- **💾 Database Management:** Uses SQLite to store manga, chapters, and user data.
- **⏰ Cron Jobs:** Automatically check for updates every 6 hours.

## Requirements 🛠️

- Go 1.23.5 or newer
- SQLite3
- Docker (optional)
- A Telegram bot token
- A .env file with the following variables:
  ```env
  TELEGRAM_BOT_TOKEN=<your_bot_token>
  TELEGRAM_ALLOWED_USERS=<comma_separated_chat_ids>
  ```

## Setting up Telegram Bot 🤖

1. **Create a Bot:**
   - Open Telegram and search for [@BotFather](https://t.me/botfather)
   - Send `/newbot` command
   - Follow instructions to name your bot
   - Save the API token provided by BotFather

2. **Get Your Chat ID:**
   - Search for [@userinfobot](https://t.me/userinfobot) on Telegram
   - Send any message to the bot
   - It will reply with your user info including your ID
   - Save this ID for the `TELEGRAM_ALLOWED_USERS` environment variable

## Installation 🖥️

1. **Clone the Repository:**

   ```bash
   git clone <repository-url>
   cd <repository-folder>
   ```

2. **Using Docker Compose:**

   - Ensure you have a `docker-compose.yml` file in your project root (one was generated for you).
   - Build and run the Docker containers:
     ```bash
     docker-compose up -d --build
     ```
   - To stop the containers:
     ```bash
     docker-compose down
     ```

3. **Install Dependencies (Manual Installation):**
   Make sure you have `go` installed. Install required Go packages:

   ```bash
   go get github.com/joho/godotenv
   go get github.com/go-telegram-bot-api/telegram-bot-api/v5
   go get github.com/robfig/cron/v3
   go get github.com/mattn/go-sqlite3
   ```

4. **Create Required Files and Directories:**

   - `.env` file (as specified in Requirements).
   - Ensure the folders `logs` and `database` exist.

5. **Run the Application:**

   ```bash
   go run main.go
   ```

## Usage 🎮

### Getting MangaDex ID 📚

1. Go to [MangaDex](https://mangadex.org) and search for your manga
2. Click on the manga title to open its page
3. The ID is in the URL, for example:
   - For URL: `https://mangadex.org/title/a1c7c817-4e59-43b7-9365-09675a149a6f/one-piece`
   - The ID is: `a1c7c817-4e59-43b7-9365-09675a149a6f`
4. Use this ID when adding a manga through the bot

### Telegram Commands 🗨️

- **/start:** Show the main menu.
- **/help:** Display help information.

### Main Menu Options 📋

- **➕ Add Manga:** Add a new manga to track by providing its MangaDex ID.
- **📚 List Followed Manga:** View all the manga you are currently tracking.
- **🔍 Check for New Chapters:** Check for updates and see newly released chapters.
- **✅ Mark Chapters as Read:** Update your progress by marking chapters as read.
- **📖 List Read Chapters:** Review chapters you've marked as read.
- **🗑️ Remove Manga:** Stop tracking a manga you no longer wish to follow.

### Notifications 📤

The bot sends updates about new chapters every 6 hours (via a cron job). You can view and manage these updates directly through Telegram.

## Code Overview 🧑‍💻

### File Structure 📂

- **main.go:** Contains the entire application logic, including:
  - Initialization of logger, folders, and database.
  - Telegram bot setup and command handling.
  - Cron job for daily updates.
  - Functions for managing manga, chapters, and user interactions.

## Logs 🗂️

All logs are stored in the `logs` directory with the filename `ReleaseNoJutsu.log`. The logs include details of application startup, database operations, user interactions, and errors.

## Contributing 🤝

1. Fork the repository.
2. Create a feature branch:
   ```bash
   git checkout -b feature-name
   ```
3. Commit your changes:
   ```bash
   git commit -m "Description of changes"
   ```
4. Push your branch:
   ```bash
   git push origin feature-name
   ```
5. Create a pull request.

## Troubleshooting 🛠️

- **Error: Missing .env file:** Ensure the `.env` file exists with the correct variables.
- **SQLite Errors:** Verify that the `database` directory is writable and SQLite3 is installed.
- **Telegram Bot Issues:** Ensure the bot token and allowed user IDs in `.env` are correct.

## License 📜

This project is licensed under the GPLv3 License.

---
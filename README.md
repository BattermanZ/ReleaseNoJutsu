# ReleaseNoJutsu 🥷

ReleaseNoJutsu is a personal manga update assistant. It tracks your favourite manga series on MangaDex and provides notifications for new chapters via Telegram. It also allows you to manage your reading progress conveniently through a Telegram bot interface.

## Features ✨

- **📖 Track Manga:** Add manga by providing its MangaDex ID.
- **🔔 Notifications:** Receive daily updates about new chapters.
- **✅ Progress Management:** Mark chapters as read or unread.
- **💾 Database Management:** Uses SQLite to store manga, chapters, and user data.
- **⏰ Cron Jobs:** Automatically check for updates every day at 7 AM.

## Requirements 🛠️

- Go 1.18 or newer
- SQLite3
- Docker (optional)
- A Telegram bot token
- A .env file with the following variables:
  ```env
  TELEGRAM_BOT_TOKEN=<your_bot_token>
  TELEGRAM_ALLOWED_USERS=<comma_separated_chat_ids>
  ```

## Installation 🖥️

1. **Clone the Repository:**

   ```bash
   git clone <repository-url>
   cd <repository-folder>
   ```

2. **Using Docker:**

   - Build the Docker image:
     ```bash
     docker build -t releasenojutsu .
     ```
   - Run the Docker container:
     ```bash
     docker run -d --name releasenojutsu \
       -v $(pwd)/logs:/app/logs \
       -v $(pwd)/database:/app/database \
       --env-file .env \
       releasenojutsu
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

### Telegram Commands 🗨️

- **/start:** Show the main menu.
- **/help:** Display help information.

### Main Menu Options 📋

- **➕ Add Manga:** Add a new manga to track by providing its MangaDex ID.
- **📚 List Followed Manga:** View all the manga you are currently tracking.
- **🔍 Check for New Chapters:** Check for updates and see newly released chapters.
- **✅ Mark Chapters as Read:** Update your progress by marking chapters as read.
- **📖 List Read Chapters:** Review chapters you've marked as read.

### Notifications 📤

The bot sends updates about new chapters daily at 7 AM (via a cron job). You can view and manage these updates directly through Telegram.

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
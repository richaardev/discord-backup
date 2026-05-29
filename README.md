


# Discord Backuper

A robust and efficient Discord bot built in Go designed to safely backup and restore your Discord server's structure. This includes roles, channels, and bans, helping you safeguard your server against accidental deletions or malicious actions.

## 🎥 Demo
[!](https://github.com/user-attachments/assets/17b6f70b-dbdb-4ec4-8e08-0e3eab9a73c0)

## 🚀 Features & Commands

The bot uses Discord's Slash Commands (`/`) for easy interaction. The main command is `/backup`, which contains the following subcommands:

- `/backup create` - Captures the current state of the server (roles, channels, bans) and saves it as a new backup.
- `/backup list` - Displays a list of all backups you have created.
- `/backup info <id>` - Shows detailed information about a specific backup (number of roles, channels, etc.).
- `/backup apply <id>` - Opens an interactive panel to apply/restore a specific backup to the server.
- `/backup delete <id>` - Permanently deletes a specific backup from the database.

## 🛠️ How to run the bot

### Prerequisites
- [Go](https://golang.org/doc/install) 1.21+ installed
- A Discord Bot Token (from the [Discord Developer Portal](https://discord.com/developers/applications))

### Installation & Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/richaardev/discord-backuper.git
   cd discord-backuper
   ```

2. **Configure environment variables:**
   Copy the example environment file and fill in your details:
   ```bash
   cp .env.example .env
   ```
   Open the `.env` file and set the required variables:
   ```env
   DISCORD_TOKEN=your_bot_token_here
   DATABASE_PATH=./data/backups.db
   ```

3. **Install dependencies:**
   ```bash
   make tidy
   ```

4. **Run the bot:**
   You can run the bot directly during development:
   ```bash
   make run
   ```
   *Alternatively, using Go directly:*
   ```bash
   go run ./cmd/bot
   ```

### Building for Production
To build a compiled binary of the bot:
```bash
make build
```
This will output the executable into the `bin/` directory, which you can run via `./bin/bot`.


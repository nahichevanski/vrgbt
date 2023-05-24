package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"test_bot/db"
	"test_bot/m"

	_ "github.com/go-sql-driver/mysql"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	bot *tgbotapi.BotAPI
)

func main() {

	var err error
	bot, err = tgbotapi.NewBotAPI("token")
	if err != nil {
		// Abort if something is wrong
		log.Panic(err)
	}

	// Set this to true to log all interactions with telegram servers
	bot.Debug = false

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	dbase, err := sql.Open("mysql", "nuser:password*@tcp(localhost:3306)/test_db")
	database := db.DB{DB: dbase}
	if err != nil {
		log.Fatal("can't open database")
	}
	defer database.Close()

	// Create a new cancellable background context. Calling `cancel()` leads to the cancellation of the context
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// `updates` is a golang channel which receives telegram updates
	updates := bot.GetUpdatesChan(u)

	// Pass cancellable context to goroutine
	go receiveUpdates(ctx, updates, &database)

	// Tell the user the bot is online
	log.Println("Start listening for updates. Press enter to stop")

	// Wait for a newline symbol, then cancel handling updates
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	cancel()

}

func receiveUpdates(ctx context.Context, updates tgbotapi.UpdatesChannel, db *db.DB) {
	// `for {` means the loop is infinite until we manually stop it
	for {
		select {
		// stop looping if ctx is cancelled
		case <-ctx.Done():
			return
		// receive update from channel and then handle it
		case update := <-updates:
			handleUpdate(update, db)
		}
	}
}

func handleUpdate(update tgbotapi.Update, db *db.DB) {

	switch {

	// Handle messages
	case update.Message != nil:
		handleMessage(update.Message, db)
	}
}

func handleMessage(message *tgbotapi.Message, db *db.DB) {
	user := message.From
	text := message.Text

	if user == nil {
		return
	}

	// Print to console
	log.Printf("%s wrote %s", user.FirstName, text)

	var err error

	//for russian letters(2 bytes)
	if len(text) > 0 {

		answer, err := handleCommand(message.Chat.ID, text, db)

		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ошибка: %v", err))
			_, err = bot.Send(msg)

		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, answer)
			_, err = bot.Send(msg)
		}

	} else {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Недостаточно данных\nили неверная команда")
		_, err = bot.Send(msg)
	}

	if err != nil {
		log.Printf("An error occured: %s", err.Error())
	}
}

// When we get a command, we react accordingly
func handleCommand(chatId int64, command string, db *db.DB) (string, error) {

	cmd := strings.ToLower(strings.TrimSpace(command))

	switch {
	//"П" - проверить количество
	case strings.HasPrefix(cmd, "п"):

		prodString, err := db.CheckQty(cmd)
		if err != nil {
			return fmt.Sprintf("Ошибка: %v", err), err
		}
		return prodString, err
	//"С" - создать
	case strings.HasPrefix(cmd, "с"):

		listID, err := db.CreateNewProdlist(cmd)
		if err != nil {
			return fmt.Sprintf("Ошибка: %v", err), err
		}
		return listID, err
	//"Д" - добавить
	case strings.HasPrefix(cmd, "д"):

		list, err := db.AddToProdlist(cmd)
		if err != nil {
			return fmt.Sprintf("Ошибка: %v", err), err
		}
		return list, err
	//"У" - удалить
	case strings.HasPrefix(cmd, "у"):

		list, err := db.RemoveFromProdlist(cmd)
		if err != nil {
			return fmt.Sprintf("Ошибка: %v", err), err
		}
		return list, err
	//"В" - вывести на экран
	case strings.HasPrefix(cmd, "в"):

		list, err := db.ShowProdlist(cmd)
		if err != nil {
			return fmt.Sprintf("Ошибка: %v", err), err
		}
		return list, err

	case command == "help" || command == "Help" || command == "/start":
		//return m.HelpMessage(), nil
		return m.HelpMsg, nil

	default:
		return "Несуществующая команда", nil
	}
}

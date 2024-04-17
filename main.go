package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
	}
	botToken := os.Getenv("TG_BOT_TOKEN")

	b, err := bot.New(botToken, opts...)
	if err != nil {
		panic(err)
	}
	_, err = b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: []models.BotCommand{
			{
				Command:     "start",
				Description: "Say hello",
			},
			{
				Command:     "download",
				Description: "Download video with the url",
			},
		},
	})
	if err != nil {
		return
	}
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		"/start",
		bot.MatchTypeExact,
		func(ctx context.Context, bot_ *bot.Bot, update *models.Update) {
			bot_.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Please specify a video url",
			})
		},
	)
	b.RegisterHandler(
		bot.HandlerTypeMessageText,
		"/download",
		bot.MatchTypeExact,
		defaultHandler,
	)

	b.Start(ctx)
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	url_ := update.Message.Text
	if strings.HasPrefix(url_, "/download ") {
		url_ = strings.Replace(url_, "/download ", "", 1)
	}
	if !isValidUrl(url_) {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Invalid URL",
		})
		return
	}
	filename, err := downloadVideo(url_)
	if err != nil {
		slog.Error("Error downloading video", "error", err)
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Something went wrong",
		})
		return
	}
	slog.Info("Downloaded file", "filename", filename)

	fileData, errReadFile := os.ReadFile(filename)
	if errReadFile != nil {
		slog.Error("Error reading file", "filename", filename, "error", errReadFile)
		return
	}
	_, err = b.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID:   update.Message.Chat.ID,
		Document: &models.InputFileUpload{Filename: filename, Data: bytes.NewReader(fileData)},
		Caption:  filename,
	})

	if err != nil {
		slog.Error("send message", slog.String("error", err.Error()))
		return
	}

	err = os.Remove(filename)
	if err != nil {
		slog.Error("remove file", slog.String("error", err.Error()))
	}
}

func isValidUrl(url_ string) bool {
	_, err := url.ParseRequestURI(url_)
	if err != nil {
		return false
	}
	return true
}

func downloadVideo(url string) (string, error) {
	cmd := exec.Command("yt-dlp", "-o", "%(title)s.%(ext)s", "--quiet", url)
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error executing yt-dlp: %w", err)
	}

	// get file name
	cmd = exec.Command("yt-dlp", "-o", "%(title)s.%(ext)s", "--print", "filename", url)
	stdOut, _ := cmd.CombinedOutput()
	filepath := string(stdOut)
	filepath = strings.Trim(filepath, "\n")
	split := strings.Split(filepath, "\n")
	filepath = split[len(split)-1]

	return filepath, nil
}

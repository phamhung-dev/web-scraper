package main

import (
	"context"
	"log"
	"os"
	"web-scraper/internal/pkg/storage"
	"web-scraper/internal/task"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalln(err)
	}
	elasticAddress := []string{os.Getenv("ELASTIC_SEARCH_URL")}
	elastic, err := storage.NewElasticSearch(elasticAddress)
	if err != nil {
		log.Fatalln(err)
	}
	if err := elastic.CreateIndex("task"); err != nil {
		log.Fatalln(err)
	}
	taskStorage := storage.NewTaskStorage(elastic)
	taskService := task.NewTaskService(taskStorage)
	if err := taskService.ScrapeTasks(context.Background()); err != nil {
		log.Fatalln(err)
	}
}

package main

import (
	"log"
	"os"

	"rinha-backend-golang/config"
	"rinha-backend-golang/gateway"
	"rinha-backend-golang/worker"
)

func main() {
	config.Init()
	mode := os.Getenv("MODE")
	if mode == "worker" {
		workerService := worker.NewWorker()
		workerService.Start()
	} else {
		apiGateway := gateway.NewAPIGateway()
		apiGateway.Start()
	}
}
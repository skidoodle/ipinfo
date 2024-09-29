package main

func main() {
	initDatabases()
	go startUpdater()
	healthCheck()
	startServer()
}

package main

func main() {
	initDatabases()
	go startUpdater()
	startServer()
}

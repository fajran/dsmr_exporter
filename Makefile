all : build/dsmr_exporter build/dsmr_exporter.rpi

build/dsmr_exporter : main.go
	go build -o $@ .

build/dsmr_exporter.rpi : main.go
	GOOS=linux GOARCH=arm GOARM=5 go build -o $@ .

cookie: cookie.go
ifdef VERSION
	go build -ldflags="-X 'main.version=$(VERSION)'" -o cookie .
else
	go build -o cookie .
endif

clean:
	rm cookie
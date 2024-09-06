module github.com/thrashdev/go-webserver

go 1.22.5

require internal/database v1.0.0

require golang.org/x/crypto v0.27.0 // indirect

replace internal/database => ./internal/database/

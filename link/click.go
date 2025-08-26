package link

import (
	"database/sql"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

var Click *sql.DB

func Setup() {
	Click = clickhouse.OpenDB(&clickhouse.Options{
		Addr: []string{os.Getenv("CLICKHOUSE_ADDRESS")},
		Auth: clickhouse.Auth{
			Database: os.Getenv("CLICKOUSE_NAME"),
			Username: os.Getenv("CLICKHOUSE_USERNAME"),
			Password: os.Getenv("CLICKHOUSE_PASSWORD"),
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: time.Second * 30,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Debug:                true,
		BlockBufferSize:      10,
		MaxCompressionBuffer: 10240,
		ClientInfo: clickhouse.ClientInfo{ // optional, please see Client info section in the README.md
			Products: []struct {
				Name    string
				Version string
			}{
				{Name: "begadangz", Version: "0.1"},
			},
		},
	})
}

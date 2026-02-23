# comindwork-go

Go client library for the [ComindWork](https://comindwork.com) (Extranet) REST API.

## Installation

```bash
go get github.com/Hukyl/comindwork-go
```

## Usage

```go
package main

import (
	"fmt"
	"log"

	comindwork "github.com/Hukyl/comindwork-go"
)

func main() {
	client := comindwork.NewClient(
		"https://extranet2.newtonideas.com/api",
		"Europe/Kiev",
	)
	client.SetAuthToken("your-auth-token")

	// List records with a filter
	records, err := client.ListScopedRecords("WORKSPACE", "TASK", comindwork.ListOptions{
		ListOfFields: "ALL",
		LimitRecords: 10,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, rec := range records {
		fmt.Println(rec.GetString("title"))
	}
}
```

## License

[MIT](LICENSE)

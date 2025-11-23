package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

type ClientReference struct {
	ID       string
	Name     string
	Base     string
	URL      templ.SafeURL
	Platform string
}

type ClientsConfig struct {
	Clients      map[string]clientData `json:"clients"`
	KindMappings map[string][]string   `json:"kindMappings"`
}

type clientData struct {
	Name     string `json:"name"`
	Base     string `json:"base"`
	Platform string `json:"platform"`
}

var (
	clientConfig ClientsConfig
)

func loadClientsConfig(configPath string) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		panic("Failed to read clients.json: " + err.Error())
	}
	if err := json.Unmarshal(data, &clientConfig); err != nil {
		panic("Failed to parse clients.json: " + err.Error())
	}
}

func generateClientList(
	kind int,
	code string,
	withModifiers ...func(ClientReference, string) string,
) []ClientReference {
	kindKey := strconv.Itoa(kind)
	clientIDs, ok := clientConfig.KindMappings[kindKey]
	if !ok {
		clientIDs = clientConfig.KindMappings["default"]
	}

	clients := make([]ClientReference, 0, len(clientIDs))
	for _, id := range clientIDs {
		clientInfo, ok := clientConfig.Clients[id]
		if !ok {
			continue
		}

		c := ClientReference{
			ID:       id,
			Name:     clientInfo.Name,
			Base:     clientInfo.Base,
			Platform: clientInfo.Platform,
		}

		url := strings.Replace(c.Base, "{code}", code, -1)
		for _, modifier := range withModifiers {
			url = modifier(c, url)
		}
		c.URL = templ.SafeURL(url)

		clients = append(clients, c)
	}

	return clients
}

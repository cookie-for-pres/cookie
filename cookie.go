package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"time"

	"github.com/olekukonko/ts"
	"golang.design/x/clipboard"
)

const CONFIG_FILE = ".config/cookie/config.json"
const SYNTAX_FILE = ".config/cookie/syntax.json"

type Config struct {
	TabStop       int          `json:"tab_stop"`
	QuitTimes     int          `json:"quit_times"`
	EmptyLineChar string       `json:"empty_line_char"`
	ColorPalette  ColorPalette `json:"color_palette"`
}

func HandleConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &Config{}, errors.New("failed to get home directory")
	}

	configFile := homeDir + "/" + CONFIG_FILE
	if _, err := os.Stat(configFile); err != nil {
		if err := os.MkdirAll(homeDir+"/.config/cookie", 0755); err != nil {
			return &Config{}, errors.New("failed to create config directory")
		}

		if err := ioutil.WriteFile(configFile, []byte(startingConfigJson), 0644); err != nil {
			return &Config{}, errors.New("failed to create config file")
		}
	}

	config := &Config{}

	file, err := os.Open(configFile)
	if err != nil {
		return &Config{}, errors.New("failed to read config file")
	}

	if err := json.NewDecoder(file).Decode(config); err != nil {
		return &Config{}, errors.New("failed to decode config file")
	}

	return config, nil
}

func HandleSyntax() ([]*EditorSyntax, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.New("failed to get home directory")
	}

	syntaxFile := homeDir + "/" + SYNTAX_FILE
	if _, err := os.Stat(syntaxFile); err != nil {
		if err := os.MkdirAll(homeDir+"/.config/cookie", 0755); err != nil {
			return nil, errors.New("failed to create config directory")
		}

		if err := ioutil.WriteFile(syntaxFile, []byte(startingSyntaxJson), 0644); err != nil {
			return nil, errors.New("failed to create config file")
		}
	}

	syntax := []*EditorSyntax{}

	file, err := ioutil.ReadFile(syntaxFile)
	if err != nil {
		return nil, errors.New("failed to read config file")
	}

	if err := json.Unmarshal(file, &syntax); err != nil {
		return nil, errors.New("failed to unmarshal config file")
	}

	return syntax, nil
}

func main() {
	var editor Editor

	config, err := HandleConfig()
	if err != nil {
		die(err)
	}

	syntax, err := HandleSyntax()
	if err != nil {
		die(err)
	}

	editor.Config = config
	editor.Syntaxes = syntax

	go func() {
		for {
			config, err := HandleConfig()
			if err != nil {
				die(err)
			}

			syntax, err := HandleSyntax()
			if err != nil {
				die(err)
			}

			editor.Config = config
			editor.Syntaxes = syntax

			time.Sleep(time.Second * 5)
		}
	}()

	if err := editor.Init(); err != nil {
		die(err)
	}
	defer editor.Close()

	if len(os.Args) > 1 {
		err := editor.OpenFile(os.Args[1])
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			die(err)
		}
	}

	editor.SetStatusMessage("Help: Ctrl-S = Save | Ctrl-Q = Quit | Ctrl-F = Find | Ctrl-D = Delete Line")

	clipboard.Init()

	go func() {
		for {
			size, _ := ts.GetSize()
			editor.ScreenCols = size.Col()
			editor.ScreenRows = size.Row() - 2

			editor.Render()
			time.Sleep(time.Millisecond * 100)
		}
	}()

	for {
		editor.Render()
		if err := editor.ProcessKey(); err != nil {
			if err == ErrQuitEditor {
				break
			}
			die(err)
		}
	}
}

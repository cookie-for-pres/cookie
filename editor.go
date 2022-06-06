package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"golang.org/x/sys/unix"
)

var version = "0.1.1"

var (
	stdinfd  = int(os.Stdin.Fd())
	stdoutfd = int(os.Stdout.Fd())
)

var ErrQuitEditor = errors.New("quit editor")

type Editor struct {
	CX, CY            int
	RX                int
	RowOffset         int
	ColOffset         int
	ScreenRows        int
	ScreenCols        int
	Rows              []*Row
	Dirty             int
	QuitCounter       int
	Filename          string
	StatusMessage     string
	StatusMessageTime time.Time
	Syntax            *EditorSyntax
	Term              *unix.Termios
	Config            *Config
	Syntaxes          []*EditorSyntax
}

func enableRawMode() (*unix.Termios, error) {
	t, err := unix.IoctlGetTermios(stdinfd, ioctlReadTermios)
	if err != nil {
		return nil, err
	}
	raw := *t
	raw.Iflag &^= unix.BRKINT | unix.INPCK | unix.ISTRIP | unix.IXON
	raw.Cflag |= unix.CS8
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cc[unix.VMIN] = 0
	raw.Cc[unix.VTIME] = 1
	if err := unix.IoctlSetTermios(stdinfd, ioctlWriteTermios, &raw); err != nil {
		return nil, err
	}
	return t, nil
}

func (e *Editor) Init() error {
	termios, err := enableRawMode()
	if err != nil {
		return err
	}

	e.Term = termios
	ws, err := unix.IoctlGetWinsize(stdoutfd, unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		if _, err = os.Stdout.Write([]byte("\x1b[999C\x1b[999B")); err != nil {
			return err
		}
		if row, col, err := getCursorPosition(); err == nil {
			e.ScreenRows = row
			e.ScreenCols = col
			return nil
		}
		return err
	}
	e.ScreenRows = int(ws.Row) - 2
	e.ScreenCols = int(ws.Col)
	return nil
}

func (e *Editor) Close() error {
	if e.Term == nil {
		return fmt.Errorf("raw mode is not enabled")
	}

	return unix.IoctlSetTermios(stdinfd, ioctlWriteTermios, e.Term)
}

type key int32

const (
	keyEnter     key = 10
	keyBackspace key = 127

	keyArrowLeft key = iota + 1000
	keyArrowRight
	keyArrowUp
	keyArrowDown
	keyDelete
	keyPageUp
	keyPageDown
	keyHome
	keyEnd
)

const (
	hlNormal uint8 = iota
	hlComment
	hlMlComment
	hlKeyword1
	hlKeyword2
	hlString
	hlNumber
	hlBoolean
	hlMatch
)

type EditorSyntax struct {
	FileType  string   `json:"filetype"`
	FileMatch []string `json:"filematch"`
	Keywords  []string `json:"keywords"`
	SCS       string   `json:"scs"`
	MCS       string   `json:"mcs"`
	MCE       string   `json:"mce"`
	Flags     struct {
		HighLightNumbers  bool `json:"highlight_numbers"`
		HighLightStrings  bool `json:"highlight_strings"`
		HighLightBooleans bool `json:"highlight_booleans"`
	} `json:"flags"`
}

type Row struct {
	idx                int
	chars              []rune
	render             string
	hl                 []uint8
	hasUnclosedComment bool
}

func ctrl(char byte) byte {
	return char & 0x1f
}

func die(err error) {
	os.Stdout.WriteString("\x1b[2J")
	os.Stdout.WriteString("\x1b[H")
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func ReadKey() (key, error) {
	buf := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil && err != io.EOF {
			return 0, err
		}
		if n > 0 {
			buf = bytes.TrimRightFunc(buf, func(r rune) bool { return r == 0 })
			switch {
			case bytes.Equal(buf, []byte("\x1b[A")):
				return keyArrowUp, nil
			case bytes.Equal(buf, []byte("\x1b[B")):
				return keyArrowDown, nil
			case bytes.Equal(buf, []byte("\x1b[C")):
				return keyArrowRight, nil
			case bytes.Equal(buf, []byte("\x1b[D")):
				return keyArrowLeft, nil
			case bytes.Equal(buf, []byte("\x1b[1~")), bytes.Equal(buf, []byte("\x1b[7~")),
				bytes.Equal(buf, []byte("\x1b[H")), bytes.Equal(buf, []byte("\x1bOH")):
				return keyHome, nil
			case bytes.Equal(buf, []byte("\x1b[4~")), bytes.Equal(buf, []byte("\x1b[8~")),
				bytes.Equal(buf, []byte("\x1b[F")), bytes.Equal(buf, []byte("\x1bOF")):
				return keyEnd, nil
			case bytes.Equal(buf, []byte("\x1b[3~")):
				return keyDelete, nil
			case bytes.Equal(buf, []byte("\x1b[5~")):
				return keyPageUp, nil
			case bytes.Equal(buf, []byte("\x1b[6~")):
				return keyPageDown, nil

			default:
				return key(buf[0]), nil
			}
		}
	}
}

func (e *Editor) MoveCursor(k key) {
	switch k {
	case keyArrowUp:
		if e.CY != 0 {
			e.CY--
		}
	case keyArrowDown:
		if e.CY < len(e.Rows) {
			e.CY++
		}
	case keyArrowLeft:
		if e.CX != 0 {
			e.CX--
		} else if e.CY > 0 {
			e.CY--
			e.CX = len(e.Rows[e.CY].chars)
		}
	case keyArrowRight:
		linelen := -1
		if e.CY < len(e.Rows) {
			linelen = len(e.Rows[e.CY].chars)
		}
		if linelen >= 0 && e.CX < linelen {
			e.CX++
		} else if linelen >= 0 && e.CX == linelen {
			e.CY++
			e.CX = 0
		}
	}

	var linelen int
	if e.CY < len(e.Rows) {
		linelen = len(e.Rows[e.CY].chars)
	}
	if e.CX > linelen {
		e.CX = linelen
	}
}

func (e *Editor) ProcessKey() error {
	k, err := ReadKey()
	if err != nil {
		return err
	}
	switch k {
	case keyEnter:
		e.InsertNewline()

	case key(ctrl('q')):
		if e.Dirty > 0 && e.QuitCounter < e.Config.QuitTimes {
			e.SetStatusMessage(
				"WARNING!!! File has unsaved changes. Press Ctrl-Q %d more times to quit.", e.Config.QuitTimes-e.QuitCounter)
			e.QuitCounter++
			return nil
		}
		os.Stdout.WriteString("\x1b[2J")
		os.Stdout.WriteString("\x1b[H")
		return ErrQuitEditor

	case key(ctrl('s')):
		n, err := e.Save()
		if err != nil {
			if err == ErrPromptCanceled {
				e.SetStatusMessage("Save aborted")
			} else {
				e.SetStatusMessage("Can't save! I/O error: %s", err.Error())
			}
		} else {
			e.SetStatusMessage("%d bytes written to disk", n)
		}

	case key(ctrl('f')):
		err := e.Find()
		if err != nil {
			if err == ErrPromptCanceled {
				e.SetStatusMessage("")
			} else {
				return err
			}
		}

	case key(ctrl('d')):
		if e.CY < len(e.Rows) {
			e.Rows = append(e.Rows[:e.CY], e.Rows[e.CY+1:]...)
		}
		e.CX = 0

		if e.CY > 0 {
			e.CY--
			e.CX = len(e.Rows[e.CY].chars)
		}

	case keyHome:
		e.CX = 0

	case keyEnd:
		if e.CY < len(e.Rows) {
			e.CX = len(e.Rows[e.CY].chars)
		}

	case keyBackspace, key(ctrl('h')):
		e.DeleteChar()

	case keyDelete:
		if e.CY == len(e.Rows)-1 && e.CX == len(e.Rows[e.CY].chars) {
			break
		}

		e.MoveCursor(keyArrowRight)
		e.DeleteChar()

	case keyPageUp:
		e.CY = e.RowOffset
		for i := 0; i < e.ScreenRows; i++ {
			e.MoveCursor(keyArrowUp)
		}
	case keyPageDown:
		e.CY = e.RowOffset + e.ScreenRows - 1
		if e.CY > len(e.Rows) {
			e.CY = len(e.Rows)
		}
		for i := 0; i < e.ScreenRows; i++ {
			e.MoveCursor(keyArrowDown)
		}

	case keyArrowUp, keyArrowDown, keyArrowLeft, keyArrowRight:
		e.MoveCursor(k)

	case key(ctrl('l')), key('\x1b'):
		break

	default:
		e.InsertChar(rune(k))
	}

	e.QuitCounter = 0
	return nil
}

func (e *Editor) DrawRows(b *strings.Builder) {
	for y := 0; y < e.ScreenRows; y++ {
		filerow := y + e.RowOffset
		if filerow >= len(e.Rows) {
			if len(e.Rows) == 0 && y == e.ScreenRows/3 {
				welcomeMsg := fmt.Sprintf("Cookie Text Editor - Version %s", version)
				if runewidth.StringWidth(welcomeMsg) > e.ScreenCols {
					welcomeMsg = UTF8Slice(welcomeMsg, 0, e.ScreenCols)
				}
				padding := (e.ScreenCols - runewidth.StringWidth(welcomeMsg)) / 2
				if padding > 0 {
					b.Write([]byte(e.Config.EmptyLineChar))
					padding--
				}
				for ; padding > 0; padding-- {
					b.Write([]byte(" "))
				}
				b.WriteString(welcomeMsg)
			} else {
				b.Write([]byte(e.Config.EmptyLineChar))
			}

		} else {
			var (
				line string
				hl   []uint8
			)
			if runewidth.StringWidth(e.Rows[filerow].render) > e.ColOffset {
				line = UTF8Slice(
					e.Rows[filerow].render,
					e.ColOffset,
					utf8.RuneCountInString(e.Rows[filerow].render))
				hl = e.Rows[filerow].hl[e.ColOffset:]
			}
			if runewidth.StringWidth(line) > e.ScreenCols {
				line = runewidth.Truncate(line, e.ScreenCols, "")
				hl = hl[:utf8.RuneCountInString(line)]
			}
			currentColor := -1
			for i, r := range []rune(line) {
				if unicode.IsControl(r) {

					sym := '?'
					if r < 26 {
						sym = '@' + r
					}
					b.WriteString("\x1b[7m")
					b.WriteRune(sym)
					b.WriteString("\x1b[m")
					if currentColor != -1 {

						b.WriteString(fmt.Sprintf("\x1b[%dm", currentColor))
					}
				} else if hl[i] == hlNormal {
					if currentColor != -1 {
						currentColor = -1
						b.WriteString("\x1b[39m")
					}
					b.WriteRune(r)
				} else {
					color := SyntaxToColor(hl[i])
					if color != currentColor {
						currentColor = color
						b.WriteString(fmt.Sprintf("\x1b[%dm", color))
					}
					b.WriteRune(r)
				}
			}
			b.WriteString("\x1b[39m")
		}
		b.Write([]byte("\x1b[K"))
		b.Write([]byte("\r\n"))
	}
}

func (e *Editor) DrawStatusBar(b *strings.Builder) {
	b.Write([]byte("\x1b[7m"))
	defer b.Write([]byte("\x1b[m"))
	filename := e.Filename
	if utf8.RuneCountInString(filename) == 0 {
		filename = "[No Name]"
	}
	dirtyStatus := ""
	if e.Dirty > 0 {
		dirtyStatus = "(modified)"
	}
	lmsg := fmt.Sprintf("%.35s - %d lines %s", filename, len(e.Rows), dirtyStatus)
	if runewidth.StringWidth(lmsg) > e.ScreenCols {
		lmsg = runewidth.Truncate(lmsg, e.ScreenCols, "...")
	}
	b.WriteString(lmsg)
	filetype := "no filetype"
	if e.Syntax != nil {
		filetype = e.Syntax.FileType
	}
	rmsg := fmt.Sprintf("%s | %d/%d", filetype, e.CY+1, len(e.Rows))
	l := runewidth.StringWidth(lmsg)
	for l < e.ScreenCols {
		if e.ScreenCols-l == runewidth.StringWidth(rmsg) {
			b.WriteString(rmsg)
			break
		}
		b.Write([]byte(" "))
		l++
	}
	b.Write([]byte("\r\n"))
}

func UTF8Slice(s string, start, end int) string {
	return string([]rune(s)[start:end])
}

func (e *Editor) DrawMessageBar(b *strings.Builder) {
	b.Write([]byte("\x1b[K"))
	msg := e.StatusMessage
	if runewidth.StringWidth(msg) > e.ScreenCols {
		msg = runewidth.Truncate(msg, e.ScreenCols, "...")
	}

	if time.Since(e.StatusMessageTime) < 5*time.Second {
		b.WriteString(msg)
	}
}

func (e *Editor) RowCxToRx(row *Row, cx int) int {
	rx := 0
	for _, r := range row.chars[:cx] {
		if r == '\t' {
			rx += (e.Config.TabStop) - (rx % e.Config.TabStop)
		} else {
			rx += runewidth.RuneWidth(r)
		}
	}
	return rx
}

func (e *Editor) RowRxToCx(row *Row, rx int) int {
	curRx := 0
	for i, r := range row.chars {
		if r == '\t' {
			curRx += (e.Config.TabStop) - (curRx % e.Config.TabStop)
		} else {
			curRx += runewidth.RuneWidth(r)
		}

		if curRx > rx {
			return i
		}
	}
	panic("unreachable")
}

func (e *Editor) Scroll() {
	e.RX = 0
	if e.CY < len(e.Rows) {
		e.RX = e.RowCxToRx(e.Rows[e.CY], e.CX)
	}

	if e.CY < e.RowOffset {
		e.RowOffset = e.CY
	}

	if e.CY >= e.RowOffset+e.ScreenRows {
		e.RowOffset = e.CY - e.ScreenRows + 1
	}

	if e.RX < e.ColOffset {
		e.ColOffset = e.RX
	}

	if e.RX >= e.ColOffset+e.ScreenCols {
		e.ColOffset = e.RX - e.ScreenCols + 1
	}
}

func (e *Editor) Render() {
	e.Scroll()

	var b strings.Builder

	b.Write([]byte("\x1b[?25l"))
	b.Write([]byte("\x1b[H"))

	e.DrawRows(&b)
	e.DrawStatusBar(&b)
	e.DrawMessageBar(&b)

	b.WriteString(fmt.Sprintf("\x1b[%d;%dH", (e.CY-e.RowOffset)+1, (e.RX-e.ColOffset)+1))

	b.Write([]byte("\x1b[?25h"))
	os.Stdout.WriteString(b.String())
}

func (e *Editor) SetStatusMessage(format string, a ...interface{}) {
	e.StatusMessage = fmt.Sprintf(format, a...)
	e.StatusMessageTime = time.Now()
}

func getCursorPosition() (row, col int, err error) {
	if _, err = os.Stdout.Write([]byte("\x1b[6n")); err != nil {
		return
	}
	if _, err = fmt.Fscanf(os.Stdin, "\x1b[%d;%d", &row, &col); err != nil {
		return
	}
	return
}

func (e *Editor) RowsToString() string {
	var b strings.Builder
	for _, row := range e.Rows {
		b.WriteString(string(row.chars))
		b.WriteRune('\n')
	}
	return b.String()
}

var ErrPromptCanceled = fmt.Errorf("user canceled the input prompt")

func (e *Editor) Prompt(prompt string, cb func(query string, k key)) (string, error) {
	var b strings.Builder
	for {
		e.SetStatusMessage(prompt, b.String())
		e.Render()

		k, err := ReadKey()
		if err != nil {
			return "", err
		}
		if k == keyDelete || k == keyBackspace || k == key(ctrl('h')) {
			if b.Len() > 0 {
				bytes := []byte(b.String())
				_, size := utf8.DecodeLastRune(bytes)
				b.Reset()
				b.WriteString(string(bytes[:len(bytes)-size]))
			}
		} else if k == key('\x1b') {
			e.SetStatusMessage("")
			if cb != nil {
				cb(b.String(), k)
			}
			return "", ErrPromptCanceled
		} else if k == keyEnter {
			if b.Len() > 0 {
				e.SetStatusMessage("")
				if cb != nil {
					cb(b.String(), k)
				}
				return b.String(), nil
			}
		} else if !unicode.IsControl(rune(k)) && !isArrowKey(k) && unicode.IsPrint(rune(k)) {
			b.WriteRune(rune(k))
		}

		if cb != nil {
			cb(b.String(), k)
		}
	}
}

func isArrowKey(k key) bool {
	return k == keyArrowUp || k == keyArrowRight ||
		k == keyArrowDown || k == keyArrowLeft
}

func (e *Editor) Save() (int, error) {
	if len(e.Filename) == 0 {
		fname, err := e.Prompt("Save as: %s (ESC to cancel)", nil)
		if err != nil {
			return 0, err
		}
		e.Filename = fname
		e.SelectSyntaxHighlight()
	}

	f, err := os.OpenFile(e.Filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := f.WriteString(e.RowsToString())
	if err != nil {
		return 0, err
	}
	e.Dirty = 0
	return n, nil
}

func (e *Editor) OpenFile(filename string) error {
	e.Filename = filename
	e.SelectSyntaxHighlight()
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Bytes()

		bytes.TrimRightFunc(line, func(r rune) bool { return r == '\n' || r == '\r' })
		e.InsertRow(len(e.Rows), string(line))
	}
	if err := s.Err(); err != nil {
		return err
	}
	e.Dirty = 0
	return nil
}

func (e *Editor) InsertRow(at int, chars string) {
	if at < 0 || at > len(e.Rows) {
		return
	}
	row := &Row{chars: []rune(chars)}
	row.idx = at
	if at > 0 {
		row.hasUnclosedComment = e.Rows[at-1].hasUnclosedComment
	}
	e.UpdateRow(row)

	e.Rows = append(e.Rows, &Row{})
	copy(e.Rows[at+1:], e.Rows[at:])
	for i := at + 1; i < len(e.Rows); i++ {
		e.Rows[i].idx++
	}
	e.Rows[at] = row
}

func (e *Editor) InsertNewline() {
	if e.CX == 0 {
		e.InsertRow(e.CY, "")
	} else {
		row := e.Rows[e.CY]
		e.InsertRow(e.CY+1, string(row.chars[e.CX:]))

		row = e.Rows[e.CY]
		row.chars = row.chars[:e.CX]
		e.UpdateRow(row)
	}
	e.CY++
	e.CX = 0
}

func (e *Editor) UpdateRow(row *Row) {
	var b strings.Builder
	col := 0
	for _, r := range row.chars {
		if r == '\t' {

			b.WriteRune(' ')
			col++

			for col%e.Config.TabStop != 0 {
				b.WriteRune(' ')
				col++
			}
		} else {
			b.WriteRune(r)
		}
	}
	row.render = b.String()
	e.UpdateHighlight(row)
}

func IsSeparator(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune(",.()+-/*=~%<>[]{}:;", r)
}

func (e *Editor) UpdateHighlight(row *Row) {
	row.hl = make([]uint8, utf8.RuneCountInString(row.render))
	for i := range row.hl {
		row.hl[i] = hlNormal
	}

	if e.Syntax == nil {
		return
	}

	prevSep := true

	var strQuote rune

	inComment := row.idx > 0 && e.Rows[row.idx-1].hasUnclosedComment

	idx := 0
	runes := []rune(row.render)
	for idx < len(runes) {
		r := runes[idx]
		prevHl := hlNormal
		if idx > 0 {
			prevHl = row.hl[idx-1]
		}

		if e.Syntax.SCS != "" && strQuote == 0 && !inComment {
			if strings.HasPrefix(string(runes[idx:]), e.Syntax.SCS) {
				for idx < len(runes) {
					row.hl[idx] = hlComment
					idx++
				}
				break
			}
		}

		if e.Syntax.MCS != "" && e.Syntax.MCE != "" && strQuote == 0 {
			if inComment {
				row.hl[idx] = hlMlComment
				if strings.HasPrefix(string(runes[idx:]), e.Syntax.MCE) {
					for j := 0; j < len(e.Syntax.MCE); j++ {
						row.hl[idx] = hlMlComment
						idx++
					}
					inComment = false
					prevSep = true
					continue
				} else {
					idx++
					continue
				}
			} else if strings.HasPrefix(string(runes[idx:]), e.Syntax.MCS) {
				for j := 0; j < len(e.Syntax.MCS); j++ {
					row.hl[idx] = hlMlComment
					idx++
				}
				inComment = true
				continue
			}
		}

		if e.Syntax.Flags.HighLightStrings {
			if strQuote != 0 {
				row.hl[idx] = hlString

				if r == '\\' && idx+1 < len(runes) {
					row.hl[idx+1] = hlString
					idx += 2
					continue
				}
				if r == strQuote {
					strQuote = 0
				}
				idx++
				prevSep = true
				continue
			} else {
				if r == '"' || r == '\'' || r == '`' {
					strQuote = r
					row.hl[idx] = hlString
					idx++
					continue
				}
			}
		}

		if e.Syntax.Flags.HighLightNumbers {
			if unicode.IsDigit(r) && (prevSep || prevHl == hlNumber) ||
				r == '.' && prevHl == hlNumber {
				row.hl[idx] = hlNumber
				idx++
				prevSep = false
				continue
			}
		}

		if e.Syntax.Flags.HighLightBooleans {
			if (r == 't' || r == 'T') && idx+3 < len(runes) && strings.ToLower(string(runes[idx:idx+4])) == "true" {
				if !(idx+4 < len(runes) && !IsSeparator(runes[idx+4])) {
					for i := 0; i < 4; i++ {
						row.hl[idx] = hlBoolean
						idx++
					}

					prevSep = false
					continue
				}
			}

			if (r == 'f' || r == 'F') && idx+4 < len(runes) && strings.ToLower(string(runes[idx:idx+5])) == "false" {
				if !(idx+5 < len(runes) && !IsSeparator(runes[idx+5])) {
					for i := 0; i < 5; i++ {
						row.hl[idx] = hlBoolean
						idx++
					}

					prevSep = false
					continue
				}
			}
		}

		if prevSep {
			keywordFound := false
			for _, kw := range e.Syntax.Keywords {
				isKeyword2 := strings.HasSuffix(kw, "|")
				if isKeyword2 {
					kw = strings.TrimSuffix(kw, "|")
				}

				end := idx + utf8.RuneCountInString(kw)
				if end <= len(runes) && kw == string(runes[idx:end]) &&
					(end == len(runes) || IsSeparator(runes[end])) {
					keywordFound = true
					hl := hlKeyword1
					if isKeyword2 {
						hl = hlKeyword2
					}
					for idx < end {
						row.hl[idx] = hl
						idx++
					}
					break
				}
			}
			if keywordFound {
				prevSep = false
				continue
			}
		}

		prevSep = IsSeparator(r)
		idx++
	}

	changed := row.hasUnclosedComment != inComment
	row.hasUnclosedComment = inComment
	if changed && row.idx+1 < len(e.Rows) {
		e.UpdateHighlight(e.Rows[row.idx+1])
	}
}

func SyntaxToColor(hl uint8) int {
	switch hl {
	case hlComment, hlMlComment:
		return 90
	case hlKeyword1:
		return 94
	case hlKeyword2:
		return 96
	case hlString:
		return 36
	case hlNumber:
		return 33
	case hlBoolean:
		return 35
	case hlMatch:
		return 32
	default:
		return 37
	}
}

func (e *Editor) SelectSyntaxHighlight() {
	e.Syntax = nil
	if len(e.Filename) == 0 {
		return
	}

	ext := filepath.Ext(e.Filename)

	for _, syntax := range e.Syntaxes {
		for _, pattern := range syntax.FileMatch {
			isExt := strings.HasPrefix(pattern, ".")
			if (isExt && pattern == ext) || (!isExt && strings.Contains(e.Filename, pattern)) {
				e.Syntax = syntax
				for _, row := range e.Rows {
					e.UpdateHighlight(row)
				}
				return
			}
		}
	}
}

func (row *Row) InsertChar(at int, c rune) {
	if at < 0 || at > len(row.chars) {
		at = len(row.chars)
	}
	row.chars = append(row.chars, 0)
	copy(row.chars[at+1:], row.chars[at:])
	row.chars[at] = c
}

func (row *Row) AppendChars(chars []rune) {
	row.chars = append(row.chars, chars...)
}

func (row *Row) DeleteChar(at int) {
	if at < 0 || at >= len(row.chars) {
		return
	}
	row.chars = append(row.chars[:at], row.chars[at+1:]...)
}

func (e *Editor) InsertChar(c rune) {
	if e.CY == len(e.Rows) {
		e.InsertRow(len(e.Rows), "")
	}
	row := e.Rows[e.CY]
	row.InsertChar(e.CX, c)
	e.UpdateRow(row)
	e.CX++
	e.Dirty++
}

func (e *Editor) DeleteChar() {
	if e.CY == len(e.Rows) {
		return
	}
	if e.CX == 0 && e.CY == 0 {
		return
	}
	row := e.Rows[e.CY]
	if e.CX > 0 {
		row.DeleteChar(e.CX - 1)
		e.UpdateRow(row)
		e.CX--
		e.Dirty++
	} else {
		prevRow := e.Rows[e.CY-1]
		e.CX = len(prevRow.chars)
		prevRow.AppendChars(row.chars)
		e.UpdateRow(prevRow)
		e.DeleteRow(e.CY)
		e.CY--
	}
}

func (e *Editor) DeleteRow(at int) {
	if at < 0 || at >= len(e.Rows) {
		return
	}
	e.Rows = append(e.Rows[:at], e.Rows[at+1:]...)
	for i := at; i < len(e.Rows); i++ {
		e.Rows[i].idx--
	}
	e.Dirty++
}

func (e *Editor) Find() error {
	savedCx := e.CX
	savedCy := e.CY
	savedColOffset := e.ColOffset
	savedRowOffset := e.RowOffset

	lastMatchRowIndex := -1
	searchDirection := 1

	savedHlRowIndex := -1
	savedHl := []uint8(nil)

	onKeyPress := func(query string, k key) {
		if len(savedHl) > 0 {
			copy(e.Rows[savedHlRowIndex].hl, savedHl)
			savedHl = []uint8(nil)
		}
		switch k {
		case keyEnter, key('\x1b'):
			lastMatchRowIndex = -1
			searchDirection = 1
			return
		case keyArrowRight, keyArrowDown:
			searchDirection = 1
		case keyArrowLeft, keyArrowUp:
			searchDirection = -1
		default:

			lastMatchRowIndex = -1
			searchDirection = 1
		}

		if lastMatchRowIndex == -1 {
			searchDirection = 1
		}

		current := lastMatchRowIndex

		for i := 0; i < len(e.Rows); i++ {
			current += searchDirection
			switch current {
			case -1:
				current = len(e.Rows) - 1
			case len(e.Rows):
				current = 0
			}

			row := e.Rows[current]
			rx := strings.Index(row.render, query)
			if rx != -1 {
				lastMatchRowIndex = current
				e.CY = current
				e.CX = e.RowRxToCx(row, rx)

				e.RowOffset = len(e.Rows)

				savedHlRowIndex = current
				savedHl = make([]uint8, len(row.hl))
				copy(savedHl, row.hl)
				for i := 0; i < utf8.RuneCountInString(query); i++ {
					row.hl[rx+i] = hlMatch
				}
				break
			}
		}
	}

	_, err := e.Prompt("Search: %s (ESC = Cancel | Enter = Confirm | Arrows = Prev/Next)", onKeyPress)
	if err == ErrPromptCanceled {
		e.CX = savedCx
		e.CY = savedCy
		e.ColOffset = savedColOffset
		e.RowOffset = savedRowOffset
	}
	return err
}

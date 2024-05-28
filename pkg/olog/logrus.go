// Copyright 2024 Outreach Corporation. All Rights Reserved.

// Description: Implements a logrus.Formatter interface which follows the format used by the charm logger.
//              Intended to be used by applications which currently use logrus and want consistently formatted
//              while using a mix of loggers (logrus and olog). This code was mostly copied from [here](1).
//
//              [1]: https://github.com/charmbracelet/log/blob/82b5630d2e68c2cf4c972a926be90149fe0c60b9/text.go "Charm Text Format"
//
//              Example usage:
//              ```l := logrus.New()
//				l.SetFormatter(NewCharmTextFormatter())
//				l.SetReportCaller(true)```
//
//              Example output:
//              `14:03:38 INFO <qss/qss.go:77> message key=value`

package olog

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	charm "github.com/charmbracelet/log"
	"github.com/muesli/termenv"

	"github.com/sirupsen/logrus"
)

const (
	separator       = "="
	indentSeparator = "  │ "
)

type logrusCharmTextFormat struct {
	styles     *charm.Styles
	timeFormat string
	re         *lipgloss.Renderer
}

// NewCharmTextFormatter creates a new logrus Formatter which uses a charm-style text format
func NewCharmTextFormatter() logrus.Formatter {
	return &logrusCharmTextFormat{
		styles:     charm.DefaultStyles(),
		timeFormat: "15:04:05",
		re:         lipgloss.NewRenderer(os.Stdout, termenv.WithColorCache(true)),
	}
}

// Format implements logrus.Formatter using a charm-style text format
func (l *logrusCharmTextFormat) Format(entry *logrus.Entry) ([]byte, error) {
	var level charm.Level
	switch entry.Level {
	case logrus.FatalLevel, logrus.PanicLevel:
		level = charm.FatalLevel
	case logrus.ErrorLevel:
		level = charm.ErrorLevel
	case logrus.WarnLevel:
		level = charm.WarnLevel
	case logrus.InfoLevel:
		level = charm.InfoLevel
	case logrus.DebugLevel, logrus.TraceLevel:
		level = charm.DebugLevel
	}
	entries := []interface{}{
		charm.TimestampKey, entry.Time,
		charm.LevelKey, level,
	}
	if entry.HasCaller() {
		entries = append(entries, charm.CallerKey, fmt.Sprintf(
			"%s/%s:%d",
			path.Base(filepath.Dir(entry.Caller.File)),
			path.Base(entry.Caller.File),
			entry.Caller.Line,
		))
	}
	entries = append(entries, charm.MessageKey, entry.Message)
	for k, v := range entry.Data {
		entries = append(entries, k, v)
	}
	b := &bytes.Buffer{}
	l.textFormatter(b, entries...)
	return b.Bytes(), nil
}

func (l *logrusCharmTextFormat) writeIndent(w io.Writer, str, indent string, newline bool, key string) {
	st := l.styles

	// kindly borrowed from hclog
	for {
		nl := strings.IndexByte(str, '\n')
		if nl == -1 {
			if str != "" {
				w.Write([]byte(indent)) // nolint:errcheck // Why: consistent with source code
				val := escapeStringForOutput(str, false)
				if valueStyle, ok := st.Values[key]; ok {
					val = valueStyle.Renderer(l.re).Render(val)
				} else {
					val = st.Value.Renderer(l.re).Render(val)
				}
				w.Write([]byte(val)) // nolint:errcheck // Why: consistent with source code
				if newline {
					w.Write([]byte{'\n'}) // nolint:errcheck // Why: consistent with source code
				}
			}
			return
		}

		w.Write([]byte(indent)) // nolint:errcheck // Why: consistent with source code
		val := escapeStringForOutput(str[:nl], false)
		val = st.Value.Renderer(l.re).Render(val)
		w.Write([]byte(val))  // nolint:errcheck // Why: consistent with source code
		w.Write([]byte{'\n'}) // nolint:errcheck // Why: consistent with source code
		str = str[nl+1:]
	}
}

func needsEscaping(str string) bool {
	for _, b := range str {
		if !unicode.IsPrint(b) || b == '"' {
			return true
		}
	}

	return false
}

const (
	lowerhex = "0123456789abcdef"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}

func escapeStringForOutput(str string, escapeQuotes bool) string {
	// kindly borrowed from hclog
	if !needsEscaping(str) {
		return str
	}

	bb := bufPool.Get().(*strings.Builder)
	bb.Reset()

	defer bufPool.Put(bb)
	for _, r := range str {
		switch {
		case escapeQuotes && r == '"':
			bb.WriteString(`\"`)
		case unicode.IsPrint(r):
			bb.WriteRune(r)
		default:
			switch r {
			case '\a':
				bb.WriteString(`\a`)
			case '\b':
				bb.WriteString(`\b`)
			case '\f':
				bb.WriteString(`\f`)
			case '\n':
				bb.WriteString(`\n`)
			case '\r':
				bb.WriteString(`\r`)
			case '\t':
				bb.WriteString(`\t`)
			case '\v':
				bb.WriteString(`\v`)
			default:
				switch {
				case r < ' ':
					bb.WriteString(`\x`)
					bb.WriteByte(lowerhex[byte(r)>>4])
					bb.WriteByte(lowerhex[byte(r)&0xF])
				case !utf8.ValidRune(r):
					r = 0xFFFD
					fallthrough
				case r < 0x10000:
					bb.WriteString(`\u`)
					for s := 12; s >= 0; s -= 4 {
						bb.WriteByte(lowerhex[r>>uint(s)&0xF])
					}
				default:
					bb.WriteString(`\U`)
					for s := 28; s >= 0; s -= 4 {
						bb.WriteByte(lowerhex[r>>uint(s)&0xF])
					}
				}
			}
		}
	}

	return bb.String()
}

func needsQuoting(s string) bool {
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			if needsQuotingSet[b] {
				return true
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return true
		}
		i += size
	}
	return false
}

var needsQuotingSet = (func() [utf8.RuneSelf]bool {
	set := [utf8.RuneSelf]bool{
		'"': true,
		'=': true,
	}
	for i := 0; i < utf8.RuneSelf; i++ {
		r := rune(i)
		if unicode.IsSpace(r) || !unicode.IsPrint(r) {
			set[i] = true
		}
	}
	return set
})()

func writeSpace(w io.Writer, first bool) {
	if !first {
		w.Write([]byte{' '}) // nolint:errcheck // Why: consistent with source code
	}
}

// nolint:funlen // Why: consistent with source code
func (l *logrusCharmTextFormat) textFormatter(b *bytes.Buffer, keyvals ...interface{}) {
	st := l.styles
	lenKeyvals := len(keyvals)

	for i := 0; i < lenKeyvals; i += 2 {
		firstKey := i == 0
		moreKeys := i < lenKeyvals-2

		switch keyvals[i] {
		case charm.TimestampKey:
			if t, ok := keyvals[i+1].(time.Time); ok {
				ts := t.Format(l.timeFormat)
				ts = st.Timestamp.Renderer(l.re).Render(ts)
				writeSpace(b, firstKey)
				b.WriteString(ts)
			}
		case charm.LevelKey:
			if level, ok := keyvals[i+1].(charm.Level); ok {
				var lvl string
				lvlStyle, ok := st.Levels[level]
				if !ok {
					continue
				}

				lvl = lvlStyle.Renderer(l.re).String()
				if lvl != "" {
					writeSpace(b, firstKey)
					b.WriteString(lvl)
				}
			}
		case charm.CallerKey:
			if caller, ok := keyvals[i+1].(string); ok {
				caller = fmt.Sprintf("<%s>", caller)
				caller = st.Caller.Renderer(l.re).Render(caller)
				writeSpace(b, firstKey)
				b.WriteString(caller)
			}
		case charm.PrefixKey:
			if prefix, ok := keyvals[i+1].(string); ok {
				prefix = st.Prefix.Renderer(l.re).Render(prefix + ":")
				writeSpace(b, firstKey)
				b.WriteString(prefix)
			}
		case charm.MessageKey:
			if msg := keyvals[i+1]; msg != nil {
				m := fmt.Sprint(msg)
				m = st.Message.Renderer(l.re).Render(m)
				writeSpace(b, firstKey)
				b.WriteString(m)
			}
		default:
			sep := separator
			indentSep := indentSeparator
			sep = st.Separator.Renderer(l.re).Render(sep)
			indentSep = st.Separator.Renderer(l.re).Render(indentSep)
			key := fmt.Sprint(keyvals[i])
			val := fmt.Sprintf("%+v", keyvals[i+1])
			raw := val == ""
			if raw {
				val = `""`
			}
			if key == "" {
				continue
			}
			actualKey := key
			valueStyle := st.Value
			if vs, ok := st.Values[actualKey]; ok {
				valueStyle = vs
			}
			if keyStyle, ok := st.Keys[key]; ok {
				key = keyStyle.Renderer(l.re).Render(key)
			} else {
				key = st.Key.Renderer(l.re).Render(key)
			}

			// Values may contain multiple lines, and that format
			// is preserved, with each line prefixed with a "  | "
			// to show it's part of a collection of lines.
			//
			// Values may also need quoting, if not all the runes
			// in the value string are "normal", like if they
			// contain ANSI escape sequences.
			switch {
			case strings.Contains(val, "\n"):
				b.WriteString("\n  ")
				b.WriteString(key)
				b.WriteString(sep + "\n")
				l.writeIndent(b, val, indentSep, moreKeys, actualKey)
			case !raw && needsQuoting(val):
				writeSpace(b, firstKey)
				b.WriteString(key)
				b.WriteString(sep)
				b.WriteString(valueStyle.Renderer(l.re).Render(fmt.Sprintf("%q",
					escapeStringForOutput(val, true))))
			default:
				val = valueStyle.Renderer(l.re).Render(val)
				writeSpace(b, firstKey)
				b.WriteString(key)
				b.WriteString(sep)
				b.WriteString(val)
			}
		}
	}

	// Add a newline to the end of the log message.
	b.WriteByte('\n')
}

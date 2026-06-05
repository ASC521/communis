package slogx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	defaultTimeFmt = time.RFC3339
	levelValues    = map[slog.Level]string{
		slog.LevelDebug: "DEBUG",
		slog.LevelInfo:  " INFO",
		slog.LevelWarn:  " WARN",
		slog.LevelError: "ERROR",
	}
)

type HandlerOptions struct {
	Level         slog.Leveler
	IncludeSource bool
}

type groupOrAttrs struct {
	group string
	attrs []slog.Attr
}

// PipeHandler was implemented following the slog Handler creation guide @ https://github.com/golang/example/tree/master/slog-handler-guide
// Additional optimizations which could be implemented if needed:
//   - Implement a buffer pool instead of creating a buffer for every log record
//   - Preformat Attrs and Group name

type PipeHandler struct {
	opts HandlerOptions
	mu   *sync.Mutex
	out  io.Writer
	goas []groupOrAttrs
}

func NewPipeHandler(out io.Writer, opts *HandlerOptions) *PipeHandler {
	h := &PipeHandler{out: out, mu: &sync.Mutex{}}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	return h
}

func (h *PipeHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.opts.Level.Level()
}

func (h *PipeHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := make([]byte, 0, 1024)
	if !r.Time.IsZero() {
		buf = fmt.Appendf(buf, "| %s |", r.Time.Format(defaultTimeFmt))
	}

	goas := h.goas
	groupNames := []string{}
	for _, goa := range goas {
		if goa.group != "" {
			groupNames = append(groupNames, goa.group)
		}
	}
	groupName := strings.Join(groupNames, ".")

	buf = fmt.Appendf(buf, " %s |", levelValues[r.Level])
	if groupName != "" {
		buf = fmt.Appendf(buf, " %s | %s | ", groupName, r.Message)
	} else {
		buf = fmt.Appendf(buf, " %s | ", r.Message)
	}

	if r.PC != 0 && h.opts.IncludeSource {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		buf = fmt.Appendf(buf, " %s:%s | ", f.File, strconv.Itoa(f.Line))
	}

	for _, goa := range goas {
		if goa.group != "" {
			continue
		}
		for _, a := range goa.attrs {
			buf = h.appendAttr(buf, a)
		}
	}

	r.Attrs(func(a slog.Attr) bool {
		buf = h.appendAttr(buf, a)
		return true
	})
	if r.NumAttrs() == 0 {
		buf = append(buf, "\n"...)
	} else {
		buf = append(buf, "|\n"...)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf)
	return err
}

// WithAttrs returns a new slog.Handler whose attributes consist of both the
// receiver's attributes and the arguments.
func (h *PipeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return h.clone(groupOrAttrs{attrs: attrs})
}

// WithGroup returns a new slog.Handler with the given group appended to the receiver's existing groups.
// They keys of all subsequent attributes, whether added by With or in a Record, should be qualified by the sequence of group names.
func (h *PipeHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.clone(groupOrAttrs{group: name})
}

func (h *PipeHandler) appendAttr(buf []byte, a slog.Attr) []byte {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return buf
	}

	switch a.Value.Kind() {
	case slog.KindString:
		buf = fmt.Appendf(buf, "%s=%q ", a.Key, a.Value.String())
	case slog.KindTime:
		buf = fmt.Appendf(buf, "%s=%s ", a.Key, a.Value.Time().Format(defaultTimeFmt))
	case slog.KindGroup:
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return buf
		}

		for _, attr := range attrs {
			buf = h.appendAttr(buf, attr)
		}
	default:
		buf = fmt.Appendf(buf, "%s=%s ", a.Key, a.Value)
	}
	return buf
}

func (h *PipeHandler) clone(goa groupOrAttrs) *PipeHandler {
	h2 := *h
	h2.goas = make([]groupOrAttrs, len(h.goas)+1)
	copy(h2.goas, h.goas)
	h2.goas[len(h2.goas)-1] = goa
	return &h2
}

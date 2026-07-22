package process

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"

	"github.com/bluehoodie/whoseport/internal/model"
)

// ErrNoService is returned when no process is listening on the requested port.
var ErrNoService = errors.New("no service listening on port")

// LsofParser parses lsof -F (field output mode) into a ProcessInfo.
type LsofParser struct{}

// NewLsofParser creates a new LsofParser.
func NewLsofParser() *LsofParser {
	return &LsofParser{}
}

// entry holds the fields lsof -F emits for a single listening file descriptor.
type entry struct {
	pid     int
	command string
	user    string
	fd      string
	typ     string
	device  string
	offset  string
	proto   string
	name    string
}

// Parse parses lsof -F output (see `man lsof`, OUTPUT FOR OTHER PROGRAMS section) and
// returns the first listening process found. lsof emits one field per line, each
// prefixed by a single identifier character: `p` starts a new process record (pid),
// `f` starts a new file record (fd) within the current process. All other identifiers
// attach to whichever record is currently open. This caller requests the `p c L f t d
// o P n` fields (pid, command, login name, fd, type, device, offset, protocol, name).
func (p *LsofParser) Parse(output []byte) (*model.ProcessInfo, error) {
	var entries []entry
	var cur *entry
	var pid int
	var command, user string

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		id, val := line[0], line[1:]
		switch id {
		case 'p':
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("could not convert process id to int: %w", err)
			}
			pid = n
			command, user = "", ""
			cur = nil
		case 'c':
			command = val
		case 'L':
			user = val
		case 'f':
			entries = append(entries, entry{pid: pid, command: command, user: user, fd: val})
			cur = &entries[len(entries)-1]
		case 't':
			if cur != nil {
				cur.typ = val
			}
		case 'd':
			if cur != nil {
				cur.device = val
			}
		case 'o':
			if cur != nil {
				cur.offset = val
			}
		case 'P':
			if cur != nil {
				cur.proto = val
			}
		case 'n':
			if cur != nil {
				cur.name = val
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read lsof output: %w", err)
	}

	if len(entries) == 0 {
		return nil, ErrNoService
	}

	e := entries[0]
	// ponytail: the executor only ever requests -sTCP:LISTEN rows, so every entry
	// here is a listener; append the suffix the rest of the app (README, display
	// layer, JSON consumers) already expects in the name field.
	return model.New(
		e.command,
		e.pid,
		e.user,
		e.fd,
		e.typ,
		e.device,
		e.offset,
		e.proto,
		e.name+" (LISTEN)",
	), nil
}

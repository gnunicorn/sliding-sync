package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const DELETED = "DELETED"

var flagFile = flag.String("file", "", "Path to the logfile with instructions")

var (
	instrRegexp      = regexp.MustCompile(`^.*(SYNC|INVALIDATE|INSERT|DELETE) [0-9]+.*;`)
	syncRegexp       = regexp.MustCompile(`SYNC ([0-9]+) ([0-9]+) ([0-9]+) (.*) ;`)
	invalidateRegexp = regexp.MustCompile(`INVALIDATE ([0-9]+) ([0-9]+)`)
	insertRegexp     = regexp.MustCompile(`INSERT ([0-9]+) ([0-9]+) (.*) ;`)
	deleteRegexp     = regexp.MustCompile(`DELETE ([0-9]+) ([0-9]+)`)
)

type Op struct {
	Name      string
	ListIndex string

	Start   string
	End     string
	RoomIDs []string

	Index  string
	RoomID string
}

func NewOpFromNoisyString(line string) *Op {
	match := insertRegexp.FindStringSubmatch(line)
	if match != nil {
		return &Op{
			Name:      "INSERT",
			ListIndex: match[1],
			Index:     match[2],
			RoomID:    match[3],
		}
	}
	match = deleteRegexp.FindStringSubmatch(line)
	if match != nil {
		return &Op{
			Name:      "DELETE",
			ListIndex: match[1],
			Index:     match[2],
		}
	}
	match = invalidateRegexp.FindStringSubmatch(line)
	if match != nil {
		return &Op{
			Name:      "INVALIDATE",
			ListIndex: match[1],
			Start:     match[2],
			End:       match[3],
		}
	}
	match = syncRegexp.FindStringSubmatch(line)
	if match != nil {
		return &Op{
			Name:      "SYNC",
			ListIndex: match[1],
			Start:     match[2],
			End:       match[3],
			RoomIDs:   strings.Split(match[4], " "),
		}
	}
	return nil
}

func extractInstructions(fname string) (instrs []string) {
	file, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 1*1024*1024)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			log.Fatalf("failed to read line: %s", err)
		}
		if err == io.EOF {
			break
		}

		if instrRegexp.Match([]byte(line)) {
			instrs = append(instrs, line)
		}
	}
	return
}

type List struct {
	id             string
	rooms          []string
	deletedIndexes map[int]struct{}
	history        []string
}

func NewList(id string) *List {
	return &List{
		id:             id,
		deletedIndexes: make(map[int]struct{}),
	}
}

func (l *List) Sync(roomIDs []string, start, end int) {
	if start != 0 {
		fmt.Printf("List %v: ignoring SYNC %v %v because we only handle 0-N currently \n", l.id, start, end)
		return
	}
	l.rooms = roomIDs
	// TODO: handle start/end
	l.history = append(l.history, fmt.Sprintf("SYNC %v %v %v %v ;", l.id, start, end, roomIDs))
}

func (l *List) Delete(index int) {
	if index >= len(l.rooms) {
		fmt.Printf("List %v: ignoring DELETE %v because it isn't part of the initial SYNC\n", l.id, index)
		return
	}
	l.rooms[index] = DELETED
	l.deletedIndexes[index] = struct{}{}
	l.history = append(l.history, fmt.Sprintf("DELETE %v %v ;", l.id, index))
}

func (l *List) Insert(index int, roomID string) {
	if index >= len(l.rooms) {
		fmt.Printf("List %v: ignoring INSERT %v %v because it isn't part of the initial SYNC\n", l.id, index, roomID)
		return
	}
	if l.rooms[index] != DELETED {
		// need to shift left or right
		if len(l.deletedIndexes) != 1 {
			log.Fatalf("List %v: cannot INSERT %v %v because that position is occupied by %v and there are no free slots", l.id, index, roomID, l.rooms[index])
		}
		deletedIndex := -1
		for j := range l.deletedIndexes {
			deletedIndex = j
		}
		delete(l.deletedIndexes, deletedIndex)
		if deletedIndex < index {
			// free slot is earlier so move everything to the left
			for i := deletedIndex; i < index; i++ {
				l.rooms[i] = l.rooms[i+1]
			}
		} else if deletedIndex > index {
			// free slot if later so move everything to the right
			for i := deletedIndex; i > index; i-- {
				l.rooms[i] = l.rooms[i-1]
			}
		}
	}

	l.rooms[index] = roomID
	l.history = append(l.history, fmt.Sprintf("INSERT %v %v %v ;", l.id, index, roomID))
}

func (l *List) DuplicateCheck() error {
	set := make(map[string]int)
	for i, roomID := range l.rooms {
		j, exists := set[roomID]
		if exists {
			return fmt.Errorf("list %v: room %v exists at both i=%v and i=%v", l.id, roomID, i, j)
		}
		set[roomID] = i
	}
	return nil
}

func toInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("not an int: %v", s)
	}
	return i
}

func main() {
	flag.Parse()
	if *flagFile == "" {
		flag.Usage()
		os.Exit(1)
	}
	var ops []Op
	instrStrs := extractInstructions(*flagFile)
	for _, line := range instrStrs {
		op := NewOpFromNoisyString(line)
		if op == nil {
			log.Fatalf("bad line: %v", line)
		}
		ops = append(ops, *op)
	}

	lists := make(map[string]*List)
	for _, op := range ops {
		l := lists[op.ListIndex]
		if l == nil {
			l = NewList(op.ListIndex)
			lists[op.ListIndex] = l
		}
		switch op.Name {
		case "SYNC":
			l.Sync(op.RoomIDs, toInt(op.Start), toInt(op.End))
		case "DELETE":
			l.Delete(toInt(op.Index))
		case "INSERT":
			l.Insert(toInt(op.Index), op.RoomID)
		}
		if err := l.DuplicateCheck(); err != nil {
			for _, h := range l.history {
				fmt.Println(h)
			}
			fmt.Println(err.Error())
		}
	}
}

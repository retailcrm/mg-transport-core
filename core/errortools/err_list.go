package errortools

// Node contains information about error in the list.
type Node struct {
	Err  error
	next *Node
	File string
	PC   uintptr
	Line int
}

// Error returns error message from the Node's Err.
func (e Node) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

// errList is an immutable linked list of error.
type errList struct {
	head *Node
	tail *Node
	len  int
}

// Push an error into the list.
func (l *errList) Push(pc uintptr, err error, file string, line int) {
	item := &Node{
		PC:   pc,
		Err:  err,
		File: file,
		Line: line,
	}

	if l.head == nil {
		l.head = item
	}

	if l.tail != nil {
		l.tail.next = item
	}

	l.tail = item
	l.len++
}

// Iterate over error list.
func (l *errList) Iterate() <-chan Node {
	c := make(chan Node)
	go func() {
		item := l.head
		for {
			if item == nil {
				close(c)
				return
			}
			c <- *item
			item = item.next
		}
	}()
	return c
}

// Len returns length of the list.
func (l *errList) Len() int {
	return l.len
}

package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dustin/go-humanize"
)

type meteredReader struct {
	io.Reader
	total    int64 // Total # of bytes transferred
	length   int64
	progress float64
	name     string
	start    time.Time
}

// Read 'overrides' the underlying io.Reader's Read method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
func (pt *meteredReader) Read(p []byte) (int, error) {
	if pt.start == *new(time.Time) {
		pt.start = time.Now()
	}

	n, err := pt.Reader.Read(p)
	if n > 0 {
		pt.total += int64(n)
		percentage := float64(pt.total) / float64(pt.length) * float64(100)

		speed := float64(pt.total) / time.Since(pt.start).Seconds()
		remainingTime := "-"
		if remaining, err := time.ParseDuration(fmt.Sprintf("%.0fs", float64(pt.length-pt.total)/speed)); err == nil {
			remainingTime = remaining.String()
		}

		is := fmt.Sprintf("\r\033[KGet %v %v/%v %.0f%%", pt.name, humanize.Bytes(uint64(pt.total)), humanize.Bytes(uint64(pt.length)), percentage)
		if percentage-pt.progress > 1 || percentage == 0 {
			is = is + fmt.Sprintf("\t\t\t%v/s %6s", humanize.Bytes(uint64(speed)), remainingTime)
			fmt.Fprint(os.Stderr, is)
			pt.progress = percentage
		}
	}

	return n, err
}

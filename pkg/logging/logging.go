package logging

import (
	"encoding/hex"
	"os"
	"strconv"
)

func Hexdump(payload []byte) error {
	dumper := hex.Dumper(os.Stderr)
	defer dumper.Close()
	if _, err := dumper.Write(payload); err != nil {
		return err
	}
	if _, err := os.Stdout.Write([]byte{'\n'}); err != nil {
		return err
	}
	return nil
}

func ShortCallerFormatter(file string, line int) string {
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	file = short
	return file + ":" + strconv.Itoa(line)
}

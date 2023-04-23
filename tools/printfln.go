package tools

import (
	"fmt"
	"io"
)

func Printfln(format string, a ...any) {
	fmt.Printf(format+"\n", a...)
}

func Fprintfln(file io.Writer, format string, a ...any) (int, error) {
	return fmt.Fprintf(file, format+"\n", a...)
}

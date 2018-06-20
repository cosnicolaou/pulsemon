package internal

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

func ReadTimestamps(filename string) error {
	rd, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer rd.Close()
	pulseCounter := 0
	fmt.Printf("pulse\tnanosecond\ttime\n")
	for {
		var ns int64
		err := binary.Read(rd, binary.LittleEndian, &ns)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		pulseCounter++
		fmt.Printf("%v\t%v\t%v\n", pulseCounter, ns, time.Unix(0, ns))
	}
	return nil
}

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Structs copied from https://cs.opensource.google/go/go/+/master:/src/syscall/ztypes_openbsd_amd64.go

type Timespec struct {
	Sec  int64
	Nsec int64
}

type Stat struct {
	Dev       uint64
	Ino       uint64
	Nlink     uint64
	Mode      uint32
	Uid       uint32
	Gid       uint32
	X__pad0   int32
	Rdev      uint64
	Size      int64
	Blksize   int64
	Blocks    int64
	Atim      Timespec
	Mtim      Timespec
	Ctim      Timespec
	X__unused [3]int64
}

func main() {
	path := "/tmp/test.txt"
	pathByte := make([]byte, len(path)+1)
	copy(pathByte, path)
	fd, fd2, errNo := syscall.Syscall(syscall.SYS_OPEN, uintptr(unsafe.Pointer(&pathByte[0])), uintptr(syscall.O_CREAT|syscall.O_RDWR), uintptr(0o644))
	if errNo != 0x0 {
		fmt.Println("error when opening the file:", errNo)
	}

	fmt.Println("fd:", fd, "r2:", fd2, "errNo:", errNo)

	payload := "Hello world"
	payloadByte := make([]byte, len(payload))
	copy(payloadByte, payload)
	r1, r2, errNo := syscall.Syscall(syscall.SYS_WRITE, fd, uintptr(unsafe.Pointer(&payloadByte[0])), uintptr(len(payloadByte)))
	if errNo != 0x0 {
		fmt.Println("error when writing to the file:", errNo)
	}

	fmt.Println("r1:", r1, "r2:", r2, "errNo:", errNo)

	stat := Stat{}
	r1, r2, errNo = syscall.Syscall(syscall.SYS_FSTAT, fd, uintptr(unsafe.Pointer(&stat)), 0)
	if errNo != 0x0 {
		fmt.Println("error when stating the file:", errNo)
	}

	fmt.Println("r1:", r1, "r2:", r2, "errNo:", errNo)
	fmt.Printf("%#v\n", stat)
}

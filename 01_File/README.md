# What The File is a File?

First of all, what is a file? In this first entry, I will try to understand what makes a file a file, and how to make one.

## Takeaways

Each of the points below should be correct for GNU/Linux, but I expect other OSes would have a similar implementation.

* each file-system object (file, folder, ...) is represented as an **inode**
* we interact with a file through a **file descriptor**, which is a process specific index number that is used to track the file and the right associated to it
![Wikipedia: inode](https://upload.wikimedia.org/wikipedia/commons/thumb/f/f8/File_table_and_inode_table.svg/1280px-File_table_and_inode_table.svg.png)
* we use **syscalls** in order to interact with any file

## Differences with my initial conception

I've already heard the phrase that on GNU/Linux, "everything is a file", but I didn't know how it exactly worked under the hood.

I initially expected that the OS had a tree structure to manage all the files, and that each file contained all the information related to it. What I mean by this is that all the metadata associated to the file are embedded within it alongside rights and name, but also author or location.

Instead, part of the metadata is accessible through an **inode**  (mode, file size, owner, ...) while file specific metadata is embedded within the file. The inode can be seen as a pointer/index to an entry in a giant array containing all the files in the system. Hence the name, a contraption of "index" and "node" ([ref. Wikipedia](https://en.wikipedia.org/wiki/Inode#Etymology)).

## Digging the standard library

My first goal is to create a file without calling high level functions, such as `os.Create`. I will not implement everything from scratch tho, and the lowest I will get to is the `Syscalls`.

The thing is that I knew nothing about how to create a file, so I had to take a peek into the standard library to "get inspired". There you can find several functions with evocative names:

```go
// syscall_linux.go
func Mkdir(path string, mode uint32) (err error) {
	return Mkdirat(AT_FDCWD, path, mode)
}

func Open(path string, mode int, perm uint32) (fd int, err error) {
	return openat(AT_FDCWD, path, mode|O_LARGEFILE, perm)
}

// zsyscall_linux_amd64.go
func openat(dirfd int, path string, flags int, mode uint32) (fd int, err error) {
	var _p0 *byte
	_p0, err = BytePtrFromString(path)
	if err != nil {
		return
	}
	r0, _, e1 := Syscall6(SYS_OPENAT, uintptr(dirfd), uintptr(unsafe.Pointer(_p0)), uintptr(flags), uintptr(mode), 0, 0)
	fd = int(r0)
	if e1 != 0 {
		err = errnoErr(e1)
	}
	return
}
```

What I can see is that I ended up in a syscall, passing parameters such as path and mode as pointers. So all I need to do is to find how to do syscalls, and what parameters to pass to it in order to create a file where I want.

On GNU/Linux, there are (at least) 2 calls that can do syscalls, namely:

```go
func Syscall(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err Errno)
func Syscall6(trap, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err Errno)
```

The only difference seems to be the number of parameters I am passing to the underlying implementation. Indeed, when I looked at the implementation of `Syscall` I could see that it is calling the same underlying function as `Syscall6`, but with zero-ed parameters:

```go
func Syscall(trap, a1, a2, a3 uintptr) (r1, r2 uintptr, err Errno) {
	runtime_entersyscall()
	r1, r2, err = RawSyscall6(trap, a1, a2, a3, 0, 0, 0)
	runtime_exitsyscall()
	return
}
```

N.B.: there seems to be a `RawSyscall` function, but it is not called due to being unsafe.

Now that I have the function, I need to know what to pass to it. Looking at the standard library's code, I can see that the first parameter is quite evocative, being `syscall.SYS_OPENAT` or `syscall.SYSOPEN`. As for the other parameters, I can use `man` to see them:

```bash
$ man open(2)
OPEN(2)         Linux Programmer's Manual               OPEN(2)

NAME
       open, openat, creat - open and possibly create a file

SYNOPSIS
       #include <sys/types.h>
       #include <sys/stat.h>
       #include <fcntl.h>

       int open(const char *pathname, int flags);
       int open(const char *pathname, int flags, mode_t mode);

       int creat(const char *pathname, mode_t mode);

       int openat(int dirfd, const char *pathname, int flags);
       int openat(int dirfd, const char *pathname, int flags, mode_t mode);

       /* Documented separately, in openat2(2): */
       int openat2(int dirfd, const char *pathname,
                   const struct open_how *how, size_t size);
```

Here I see `pathname` (which should correspond to where I want to create the file), `flags` (e.g. create if not present, append, read only, read-write, ...) and `mode` (e.g. o644 for what I need to do). For those 3 parameters `syscall.Syscall` should suffice, but I could have used `syscall.Syscall6` with the remaining 3 parameters set as `0`.

## File, made by hand

Now that I have all the information, creating the file is quite straightforward. Indeed, I only need 4 lines of code to create a file:

```go
func main() {
	path := "/tmp/test.txt"
	pathByte := make([]byte, len(path)+1)
	copy(pathByte, path)
	r1, r2, errNo := syscall.Syscall(syscall.SYS_OPEN, uintptr(unsafe.Pointer(&pathByte[0])), uintptr(syscall.O_CREAT|syscall.O_RDWR), uintptr(0o644))

	fmt.Println(r1, r2, errNo)
}
```

One interesting thing to note is how I pass the `pathname` parameter to the syscall. The syscall needs a pointer to a null terminated string of chars, which is why I have to initialize `pathByte` with one more byte for the final `0x00`.

Then it is only a matter of copying the string's content into my byte array. It is interesting to note that copying a `string` to `[]byte` is a special case of `copy`, I wonder if this was made so for compatibility with lower level languages...

After that it is only a matter of converting each parameter into the right type, using `unsafe.Pointer` to pass the pathname around. The Godoc for this function has a section specifically for sycall that reads:

> If a pointer argument must be converted to uintptr for use as an argument, that conversion must appear in the call expression itself

We can see 2 examples alongside the rule, with:
```go
// the good way
syscall.Syscall(SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(n))

// the bad way
u := uintptr(unsafe.Pointer(p))
syscall.Syscall(SYS_READ, uintptr(fd), u, uintptr(n))
```

Hopefully, I implemented my file creation in the good way, so the Go police wont come to my home.

If I run my script, I can see the file created in the proper folder:
```bash
❯ ls -al /tmp/test.txt
-rw-r--r-- 1 renaud renaud 0 févr. 18 18:44 /tmp/test.txt
```

### File, written by hand

Now that I have a file, I want to write to it. In order to do that I'll need a `file descriptor (fd)`. The `fd` allows a process to access a file with specific rights. Each `fd` is specific to a process, and will basically tell: "ok, this fd 3 can write to this inode".

I can also look at `write`'s manual page:

```bash
❯ man "write(2)"
WRITE(2)                Linux Programmer's Manual           WRITE(2)

NAME
       write - write to a file descriptor

SYNOPSIS
       #include <unistd.h>

       ssize_t write(int fd, const void *buf, size_t count);
```

Here again I'll have to use a `syscall`, basically asking the OS "can you write `count` bytes from this `buf` to this file descriptor `fd`?".

But where do I get the file descriptor? It "just" happens that I already have it when I initially created the file, as per `man "open(2)"`:

> The  return value of open() is a file descriptor, a small, nonnegative integer that is used in subsequent system calls (read(2), write(2), lseek(2), fcntl(2), etc.) to refer to the open file.

Okay, let's write something in this file!

```go
payload := "Hello world"
payloadByte := make([]byte, len(payload)+1)
copy(payloadByte, payload)
r1, r2, errNo := syscall.Syscall(syscall.SYS_WRITE, fd, uintptr(unsafe.Pointer(&payloadByte[0])), uintptr(len(payloadByte)))
if errNo != 0x0 {
  fmt.Println("error when writing to the file:", errNo)
}

fmt.Println("r1:", r1, "r2:", r2, "errNo:", errNo)
```

It executes, but when I want to open it with my editor:

```bash
The file is not displayed in the text editor because it is either binary or uses an unsupported text encoding.
```

Oops! I was too fast with my copy-paste, and the buffer is null terminated! All is good after removing the `+1` when initializing the buffer.

```go
payload := "Hello world"
payloadByte := make([]byte, len(payload))
copy(payloadByte, payload)
r1, r2, errNo := syscall.Syscall(syscall.SYS_WRITE, fd, uintptr(unsafe.Pointer(&payloadByte[0])), uintptr(len(payloadByte)))
if errNo != 0x0 {
    fmt.Println("error when writing to the file:", errNo)
}

fmt.Println("r1:", r1, "r2:", r2, "errNo:", errNo)
```

And I can see the most typed message of all history in my file!

```bash
❯ cat /tmp/test.txt
Hello world%
```

(I'm leaving the `%` at the end for now, but it could be interesting to investigate)

### Stat it

Now that I have created my file I want to try to stat it. As per usual, there is a syscall and a manual entry:

```bash
❯ man "stat(2)"
STAT(2)                Linux Programmer's Manual              STAT(2)

NAME
       stat, fstat, lstat, fstatat - get file status

SYNOPSIS
       #include <sys/types.h>
       #include <sys/stat.h>
       #include <unistd.h>

       int stat(const char *pathname, struct stat *statbuf);
       int fstat(int fd, struct stat *statbuf);
       int lstat(const char *pathname, struct stat *statbuf);

       #include <fcntl.h>           /* Definition of AT_* constants */
       #include <sys/stat.h>

       int fstatat(int dirfd, const char *pathname, struct stat *statbuf,
                   int flags);
```

But what is this second parameter, `statbuf`? It is a structure containing the metadata about the file:

```bash
The stat structure
    All of these system calls return a stat structure, which contains the following fields:

        struct stat {
            dev_t     st_dev;         /* ID of device containing file */
            ino_t     st_ino;         /* Inode number */
            mode_t    st_mode;        /* File type and mode */
            nlink_t   st_nlink;       /* Number of hard links */
            uid_t     st_uid;         /* User ID of owner */
            gid_t     st_gid;         /* Group ID of owner */
            dev_t     st_rdev;        /* Device ID (if special file) */
            off_t     st_size;        /* Total size, in bytes */
            blksize_t st_blksize;     /* Block size for filesystem I/O */
            blkcnt_t  st_blocks;      /* Number of 512B blocks allocated */

            /* Since Linux 2.6, the kernel supports nanosecond
                precision for the following timestamp fields.
                For the details before Linux 2.6, see NOTES. */

            struct timespec st_atim;  /* Time of last access */
            struct timespec st_mtim;  /* Time of last modification */
            struct timespec st_ctim;  /* Time of last status change */

        #define st_atime st_atim.tv_sec      /* Backward compatibility */
        #define st_mtime st_mtim.tv_sec
        #define st_ctime st_ctim.tv_sec
        };
```

So... I'll probably need to prepare a structure and pass it to the syscall so it can fill it with information, but I'm not sure how to create the structure. The order of each field could be architecture dependent and there could be padding between fields. So what I did was a bit of cheating, by looking at the existing implementation in the syscall library:

```go
func Fstat(fd int, stat *Stat_t) (err error) {
	_, _, e1 := Syscall(SYS_FSTAT, uintptr(fd), uintptr(unsafe.Pointer(stat)), 0)
	if e1 != 0 {
		err = errnoErr(e1)
	}
	return
}
```

Mkay, so all I need is to retrieve this structure that is defined for my architecture:

```go
// ztypes_linux_amd64.go
type Stat_t struct {
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
```

Now I will just stat my newly created file with:

```go
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

[...] // create file and write to file

stat := Stat{}
r1, r2, errNo = syscall.Syscall(syscall.SYS_FSTAT, fd, uintptr(unsafe.Pointer(&stat)), 0)
if errNo != 0x0 {
    fmt.Println("error when stating the file:", errNo)
}

fmt.Println("r1:", r1, "r2:", r2, "errNo:", errNo)
fmt.Printf("%#v\n", stat)
```

Executing the whole script will print:

```bash
main.Stat{Dev:0x10303, Ino:0x3980024, Nlink:0x1, Mode:0x81a4, Uid:0x3e8, Gid:0x3e8, X__pad0:0, Rdev:0x0, Size:11, Blksize:4096, Blocks:8, Atim:main.Timespec{Sec:1708279129, Nsec:808312510}, Mtim:main.Timespec{Sec:1708280176, Nsec:211117908}, Ctim:main.Timespec{Sec:1708280176, Nsec:211117908}, X__unused:[3]int64{0, 0, 0}}
```

`Mode` can be rewritten in hex as `0x100644`, the `Size` is 11, which corresponds to `Hello world` and the dates seems to correspond to when I run the script:

```bash
❯ date -d @1708280176
dim. 18 févr. 2024 19:16:16 CET
```

(dim. = Dimanche = Sunday in French)

I initially used a Stat struct from `ztypes_openbsd_amd64.go`, but the field order and the padding was different, so it was returning me garbage. I have to learn to read the filename before copying from it...

# Wrapping up

Writing a file myself with syscall was quite a fun experience, since I'm exploring areas that I don't often see in my day to day job. Once you get the hang of syscalls they are pretty straightforward to use, which is probably why most of the syscall functions does not have comments (on the other hand, it depends on the syscall you are trying to do, but I didn't know that when I started).
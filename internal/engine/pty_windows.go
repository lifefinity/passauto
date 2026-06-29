//go:build windows

package engine

import (
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32                         = syscall.NewLazyDLL("kernel32.dll")
	procCreatePseudoConsole          = kernel32.NewProc("CreatePseudoConsole")
	procResizePseudoConsole          = kernel32.NewProc("ResizePseudoConsole")
	procClosePseudoConsole           = kernel32.NewProc("ClosePseudoConsole")
	procInitializeProcThreadAttrList = kernel32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttribute    = kernel32.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttrList     = kernel32.NewProc("DeleteProcThreadAttributeList")
)

const (
	_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE = 0x00020016
	_EXTENDED_STARTUPINFO_PRESENT        = 0x00080000
)

type coord struct {
	X, Y int16
}

func runWithPTY(opts Options, matcher *Matcher) (int, error) {
	// Create pipes for PTY communication
	var ptyIn, ptyOut syscall.Handle
	var pipeIn, pipeOut syscall.Handle

	// ptyIn: we write to pipeIn, PTY reads from ptyIn
	if err := syscall.CreatePipe(&ptyIn, &pipeIn, nil, 0); err != nil {
		return 1, fmt.Errorf("creating input pipe: %w", err)
	}
	// ptyOut: PTY writes to ptyOut, we read from pipeOut
	if err := syscall.CreatePipe(&pipeOut, &ptyOut, nil, 0); err != nil {
		syscall.CloseHandle(ptyIn)
		syscall.CloseHandle(pipeIn)
		return 1, fmt.Errorf("creating output pipe: %w", err)
	}

	// Create pseudo console
	size := coord{X: 120, Y: 30}
	var hPC syscall.Handle
	r, _, err := procCreatePseudoConsole.Call(
		uintptr(*(*int32)(unsafe.Pointer(&size))),
		uintptr(ptyIn),
		uintptr(ptyOut),
		0,
		uintptr(unsafe.Pointer(&hPC)),
	)
	if r != 0 {
		syscall.CloseHandle(ptyIn)
		syscall.CloseHandle(pipeIn)
		syscall.CloseHandle(pipeOut)
		syscall.CloseHandle(ptyOut)
		return 1, fmt.Errorf("CreatePseudoConsole failed: %v (HRESULT=0x%x)", err, r)
	}

	// Close PTY-side pipe handles (PTY owns them now)
	syscall.CloseHandle(ptyIn)
	syscall.CloseHandle(ptyOut)

	// Create stepper for post-login commands (will be wired up after process starts)
	// pipeIn writer is set inside startProcessWithPTY

	// Start process with pseudo console
	cmdLine := buildCommandLine(opts.Command)
	exitCode, procErr := startProcessWithPTY(hPC, cmdLine, pipeIn, pipeOut, opts, matcher)

	if procErr != nil {
		return 1, procErr
	}
	return exitCode, nil
}

func startProcessWithPTY(hPC syscall.Handle, cmdLine string, pipeIn, pipeOut syscall.Handle, opts Options, matcher *Matcher) (int, error) {
	// Initialize thread attribute list
	var attrListSize uintptr
	procInitializeProcThreadAttrList.Call(0, 1, 0, uintptr(unsafe.Pointer(&attrListSize)))

	attrList := make([]byte, attrListSize)
	r, _, err := procInitializeProcThreadAttrList.Call(
		uintptr(unsafe.Pointer(&attrList[0])),
		1, 0,
		uintptr(unsafe.Pointer(&attrListSize)),
	)
	if r == 0 {
		return 1, fmt.Errorf("InitializeProcThreadAttributeList: %v", err)
	}
	defer procDeleteProcThreadAttrList.Call(uintptr(unsafe.Pointer(&attrList[0])))

	// Set pseudo console attribute
	r, _, err = procUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(&attrList[0])),
		0,
		_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		uintptr(hPC),
		unsafe.Sizeof(hPC),
		0, 0,
	)
	if r == 0 {
		return 1, fmt.Errorf("UpdateProcThreadAttribute: %v", err)
	}

	// Create process
	cmdLinePtr, _ := syscall.UTF16PtrFromString(cmdLine)
	var si syscall.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))

	type startupInfoEx struct {
		syscall.StartupInfo
		lpAttributeList unsafe.Pointer
	}
	siEx := startupInfoEx{
		StartupInfo:     si,
		lpAttributeList: unsafe.Pointer(&attrList[0]),
	}
	siEx.Cb = uint32(unsafe.Sizeof(siEx))

	var pi syscall.ProcessInformation
	var envBlock *uint16
	if len(opts.Env) > 0 {
		envBlock = createEnvBlock(append(os.Environ(), opts.Env...))
	}
	createErr := syscall.CreateProcess(
		nil,
		cmdLinePtr,
		nil, nil,
		false,
		_EXTENDED_STARTUPINFO_PRESENT|syscall.CREATE_UNICODE_ENVIRONMENT,
		envBlock, nil,
		&siEx.StartupInfo,
		&pi,
	)
	if createErr != nil {
		return 1, fmt.Errorf("CreateProcess: %w", createErr)
	}
	defer syscall.CloseHandle(pi.Thread)
	defer syscall.CloseHandle(pi.Process)

	// Wrap handles in os.File (one per handle — avoid double-wrapping)
	pipeWriter := os.NewFile(uintptr(pipeIn), "conpty-in")
	pipeReader := os.NewFile(uintptr(pipeOut), "conpty-out")

	// Create stepper for post-login step execution
	stepper := NewStepper(opts.Steps, opts.Prompt, pipeWriter)

	// I/O goroutines
	var wg sync.WaitGroup

	// Forward user input to PTY
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(pipeWriter, opts.Stdin)
	}()

	// Read PTY output, match patterns, forward to stdout
	wg.Add(1)
	go func() {
		defer wg.Done()

		lineBuf := ""
		buf := make([]byte, 4096)
		timer := time.NewTimer(100 * time.Millisecond)
		timer.Stop()

		type readResult struct {
			n   int
			err error
		}
		readCh := make(chan readResult, 1)

		startRead := func() {
			go func() {
				n, err := pipeReader.Read(buf)
				readCh <- readResult{n, err}
			}()
		}

		startRead()
		for {
			select {
			case res := <-readCh:
				if res.n > 0 {
					chunk := string(buf[:res.n])
					opts.Stdout.Write(buf[:res.n])
					lineBuf += chunk

					for {
						idx := findNewline(lineBuf)
						if idx < 0 {
							timer.Reset(100 * time.Millisecond)
							break
						}
						line := lineBuf[:idx+1]
						lineBuf = lineBuf[idx+1:]
						processLine(matcher, stepper, line, pipeWriter)
					}
				}
				if res.err != nil {
					if lineBuf != "" {
						processLine(matcher, stepper, lineBuf, pipeWriter)
					}
					timer.Stop()
					return
				}
				startRead()
			case <-timer.C:
				if lineBuf != "" {
					processLine(matcher, stepper, lineBuf, pipeWriter)
					lineBuf = ""
				}
			}
		}
	}()

	// Wait for process to exit
	syscall.WaitForSingleObject(pi.Process, syscall.INFINITE)
	var exitCode uint32
	syscall.GetExitCodeProcess(pi.Process, &exitCode)

	// Close pseudo console — this causes the reader pipe to get EOF
	procClosePseudoConsole.Call(uintptr(hPC))

	wg.Wait()

	pipeWriter.Close()
	pipeReader.Close()

	return int(exitCode), nil
}

func processLine(matcher *Matcher, stepper *Stepper, line string, w io.Writer) {
	clean := stripAnsi(line)
	result := matcher.Check(clean)
	if result != nil {
		w.Write([]byte(result.Response + "\r\n"))
		stepper.Activate()
		return
	}
	stepper.Check(clean)
}

func findNewline(s string) int {
	for i, c := range s {
		if c == '\n' || c == '\r' {
			return i
		}
	}
	return -1
}

func buildCommandLine(args []string) string {
	if len(args) == 0 {
		return ""
	}
	cmdLine := args[0]
	for _, arg := range args[1:] {
		cmdLine += " " + arg
	}
	return cmdLine
}

func createEnvBlock(env []string) *uint16 {
	// Environment block: "key=value\0key=value\0\0" in UTF-16
	var block []uint16
	for _, s := range env {
		u := syscall.StringToUTF16(s)
		block = append(block, u...)
	}
	block = append(block, 0) // double null terminator
	return &block[0]
}

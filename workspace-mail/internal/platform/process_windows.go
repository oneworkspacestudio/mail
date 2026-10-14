//go:build windows

package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"io"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

type Job struct{ handle windows.Handle }

func NewJob() (*Job, error) {
	h, e := windows.CreateJobObject(nil, nil)
	if e != nil {
		return nil, e
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, e = windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if e != nil {
		windows.CloseHandle(h)
		return nil, e
	}
	return &Job{h}, nil
}
func (j *Job) Close() {
	if j != nil && j.handle != 0 {
		windows.CloseHandle(j.handle)
		j.handle = 0
	}
}
func (j *Job) Start(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW}
	if e := cmd.Start(); e != nil {
		return e
	}
	fail := func(e error) error { _ = cmd.Process.Kill(); _ = cmd.Wait(); return e }
	h, e := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if e != nil {
		return fail(e)
	}
	e = windows.AssignProcessToJobObject(j.handle, h)
	windows.CloseHandle(h)
	if e != nil {
		return fail(e)
	}
	snap, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if e != nil {
		return fail(e)
	}
	defer windows.CloseHandle(snap)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	e = windows.Thread32First(snap, &entry)
	for e == nil {
		if entry.OwnerProcessID == uint32(cmd.Process.Pid) {
			thread, e := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if e != nil {
				return fail(e)
			}
			_, e = windows.ResumeThread(thread)
			windows.CloseHandle(thread)
			if e != nil {
				return fail(e)
			}
			return nil
		}
		e = windows.Thread32Next(snap, &entry)
	}
	return fail(errors.New("could not resume application primary thread"))
}

// Verify and hold a deny-write/delete handle through process lifetime. Program
// storage can be exFAT, while trusted state and the manager remain on NTFS.
func LockedBinary(path, expected string) (*os.File, error) {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, e
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(h), path)
	sum := sha256.New()
	_, e = io.Copy(sum, f)
	if e != nil || hex.EncodeToString(sum.Sum(nil)) != expected {
		f.Close()
		return nil, errors.New("application checksum does not match protected catalog")
	}
	_, e = f.Seek(0, 0)
	if e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func Lock(path string) (*os.File, error) {
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, e
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return nil, errors.New("Workspace is already running or being configured")
	}
	return os.NewFile(uintptr(h), path), nil
}
func OpenBrowser(url string) {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}
func Autostart(exe, data string, enabled bool) error {
	k, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	if !enabled {
		e = k.DeleteValue("OneWorkspace")
		if e == registry.ErrNotExist {
			return nil
		}
		return e
	}
	return k.SetStringValue("OneWorkspace", `"`+exe+`" --data "`+data+`" run`)
}

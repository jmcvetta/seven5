package util

import (
	"os"
	"strings"
	"fmt"
)

//Receiver for directory change events
type DirectoryListener interface {
	FileChanged(fileInfo os.FileInfo)
	FileRemoved(fileInfo os.FileInfo)
	FileAdded(fileInfo os.FileInfo)
}

type DirectoryMonitor struct {
	Path         string
	Extension    string
	listeners    []DirectoryListener
	previousPoll []os.FileInfo
}

func (dirMon *DirectoryMonitor) isIn(file os.FileInfo, poll []os.FileInfo) bool {
	for _, info := range poll {
		if file.Name() == info.Name() {
			return true
		}
	}
	//fmt.Printf("comparing is In: fail on %v\n",file.Name())
	return false
}

func (dirMon *DirectoryMonitor) changed(file os.FileInfo, poll []os.FileInfo) bool {
	for _, info := range poll {
		if file.Name() == info.Name() {
			//fmt.Printf("nano %s vs %s, %v\n",file.Name(),info.Name(),!file.ModTime().Equal(info.ModTime()))
			return !file.ModTime().Equal(info.ModTime())
		}
	}
	return false
}

func (dirMon *DirectoryMonitor) Poll() (changed bool, err error) {
	var dir *os.File
	fmt.Printf("POLL!\n")
	dir, err = os.Open(dirMon.Path)
	if err != nil {
		return
	}
	var currentPoll []os.FileInfo
	currentPoll, err = dir.Readdir(-1)
	if err != nil {
		return
	}
	
	if dirMon.previousPoll == nil {
		dirMon.previousPoll = currentPoll
		return
	}
	var info os.FileInfo
	for _, info = range currentPoll {
		if !strings.HasSuffix(info.Name(), dirMon.Extension) {
			continue
		}
		if !dirMon.isIn(info, dirMon.previousPoll) {
			changed = true
			for _, listener := range dirMon.listeners {
				listener.FileAdded(info)
			}
		} else if dirMon.changed(info, dirMon.previousPoll) {
			changed = true
			for _, listener := range dirMon.listeners {
				listener.FileChanged(info)
			}
		}
	}
	for _, info = range dirMon.previousPoll {
		if !strings.HasSuffix(info.Name(), dirMon.Extension) {
			continue
		}
		
		if !dirMon.isIn(info, currentPoll) {
			changed = true
			for _, listener := range dirMon.listeners {
				listener.FileRemoved(info)
			}
		}
	}
	dirMon.previousPoll = currentPoll
	return
}
func (dirMon *DirectoryMonitor) Listen(listener DirectoryListener) {
	dirMon.listeners = append(dirMon.listeners, listener)
}
func (dirMon *DirectoryMonitor) StopListening(listener *DirectoryListener) {}

func NewDirectoryMonitor(path string, extension string) (monitor *DirectoryMonitor, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if !info.IsDir() {
		return
	}
	monitor = &DirectoryMonitor{Path: path, Extension: extension}
	//this is really unclear... I can see arguments for either behavior
	//here...this insures that there are no "creation messages" just because
	//you start listening
	monitor.Poll()
	return
}
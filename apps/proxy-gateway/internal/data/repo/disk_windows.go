package repo

import "golang.org/x/sys/windows"

func diskUsage(path string) (uint64, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var available, total uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &available, &total, nil); err != nil {
		return 0, err
	}
	return usedPercent(available, total), nil
}

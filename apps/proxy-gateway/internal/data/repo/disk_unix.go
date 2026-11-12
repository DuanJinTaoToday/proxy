//go:build linux || darwin

package repo

import "golang.org/x/sys/unix"

func diskUsage(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return usedPercent(uint64(stat.Bavail), uint64(stat.Blocks)), nil
}

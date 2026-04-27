package modules

import "os"

func existsOnDisk(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

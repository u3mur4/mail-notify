package i3lock

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/fogleman/gg"
)

func sendSignalToProcess(name string) error {
	// Find the PID of the process with the given name
	pids, err := findPIDByName(name)
	if err != nil {
		return err
	}

	// Send the SIGUSR1 signal to the process
	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		err = process.Signal(syscall.SIGUSR1)
		if err != nil {
			return err
		}
	}

	return nil
}

func findPIDByName(name string) (pids []int, err error) {
	// Run the pidof command to find the PID of the process with the given name
	cmd := exec.Command("pidof", name)
	output, err := cmd.Output()
	if err != nil {
		return pids, fmt.Errorf("error running pidof: %v", err)
	}

	// Parse the output of the command to find the PID
	nums := strings.Split(string(output), " ")
	for _, num := range nums {
		pid, err := strconv.Atoi(strings.TrimSpace(num))
		if err != nil {
			// fmt.Println(fmt.Errorf("error parsing PID: %v", err))
			continue
		}
		pids = append(pids, pid)
	}

	return pids, nil
}

func GenerateImage(account string, unseen uint32, x, y int, notifyI3Lock bool) {
	dc := gg.NewContext(80, 50)
	dc.SetRGBA(1, 1, 1, 0)
	dc.Clear()
	dc.LoadFontFace("/usr/share/fonts/TTF/Noto-Sans-Regular-Nerd-Font-Complete.ttf", 50)
	mailIcon := ""
	dc.SetRGB(1, 1, 1)
	dc.DrawStringAnchored(mailIcon, 5, 3, 0, 1)
	w, h := dc.MeasureString(mailIcon)

	if unseen > 0 {
		dc.SetRGB(1, 0, 0)
		dc.DrawCircle(w+10, h/2, h/2)
		dc.Fill()

		dc.SetRGB(1, 1, 1)
		dc.LoadFontFace("/usr/share/fonts/TTF/Noto-Sans-Regular-Nerd-Font-Complete.ttf", 30)
		dc.DrawStringAnchored(fmt.Sprintf("%d", unseen), w+10, h/2, 0.5, 0.5)
	}

	os.Mkdir("/tmp/i3lock", 0755)
	dc.SavePNG(fmt.Sprintf("/tmp/i3lock/%s-pos:%d-%d.png", account, x, y))

	if notifyI3Lock {
		sendSignalToProcess("i3lock")
	}
}

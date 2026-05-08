package clipboard

import (
	"fmt"
	"os/exec"
)

func Write(text string) error {
	args, ok := command()
	if !ok {
		return fmt.Errorf("clipboard command not found")
	}
	cmd := exec.Command(args[0], args[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte(text)); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return err
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func command() ([]string, bool) {
	candidates := [][]string{
		{"pbcopy"},
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate[0]); err == nil {
			return candidate, true
		}
	}
	return nil, false
}

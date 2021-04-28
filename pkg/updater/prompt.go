package updater

import (
	"context"
	"strings"

	"github.com/manifoldco/promptui"
)

func GetYesOrNoInput(ctx context.Context) (bool, error) {
	prompt := promptui.Select{
		Label: "Select",
		Items: []string{"Yes", "No"},
	}

	_, resp, err := prompt.Run()
	if err != nil {
		return false, err
	}

	if strings.EqualFold(resp, "yes") {
		return true, nil
	}

	return false, nil
}

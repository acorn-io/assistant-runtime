package chat

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ErrNoChoice error

func choose(msg string, names, descriptions []string) (result string, _ error) {
	switch len(names) {
	case 0:
		return "", (ErrNoChoice)(fmt.Errorf("%s: none avaliable", msg))
	case 1:
		return names[0], nil
	}

	for i, desc := range descriptions {
		if desc == "" {
			descriptions[i] = names[i]
		}
	}

	err := survey.AskOne(&survey.Select{
		Message: msg,
		Options: descriptions,
		Default: descriptions[0],
	}, &result)
	for i, desc := range descriptions {
		if result == desc {
			result = names[i]
		}
	}
	return result, err
}

func selectItem(list runtime.Object, prompt string) (string, error) {
	names, descriptions, err := toChoice(list)
	if err != nil {
		return "", err
	}
	return choose(prompt, names, descriptions)
}

func toChoice(list runtime.Object) (names []string, descriptions []string, _ error) {
	err := meta.EachListItem(list, func(obj runtime.Object) error {
		var (
			desc string
		)
		if d, ok := obj.(interface {
			GetDescription() string
		}); ok {
			desc = d.GetDescription()
		}
		names = append(names, obj.(kclient.Object).GetName())
		descriptions = append(descriptions, desc)
		return nil
	})
	return names, descriptions, err
}

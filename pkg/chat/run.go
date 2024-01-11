package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	v1 "github.com/acorn-io/assistant-runtime/pkg/apis/assistant.acorn.io/v1"
	"github.com/acorn-io/baaah/pkg/name"
	"github.com/acorn-io/baaah/pkg/router"
	"github.com/acorn-io/baaah/pkg/watcher"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Options struct {
	Assistant string
	Thread    string
	Namespace string
}

func (o Options) complete() Options {
	if o.Namespace == "" {
		o.Namespace = "acorn"
	}
	return o
}

func Run(ctx context.Context, k kclient.WithWatch, opt Options) error {
	opt = opt.complete()
	r := run{
		Options: opt,
		c:       k,
	}
	return r.run(ctx)
}

type run struct {
	Options
	c      kclient.WithWatch
	helped bool
}

func (r *run) getPrompt(t *v1.Thread) string {
	if r.helped {
		return ""
	}
	var suffix string
	if t.Name != "" {
		suffix = ":" + t.Name
	}
	return "Enter text to send to assistant [" + t.Spec.AssistantName + suffix + "]"
}

func (r *run) getHelp() string {
	if !r.helped {
		//r.helped = true
		return "Enter the text to send to the assistant or enter one of the following commands\n\n" +
			"r) Resume an existing thread\n" +
			"n) Create an new thread\n" +
			"a) Switch assistant\n" +
			"q) Quit/Exit\n" +
			"d) Delete a message and then resume the thread\n" +
			"dt) Delete a thread and create a new one\n"
	}
	return ""
}

func (r *run) deleteThread(ctx context.Context, thread *v1.Thread) error {
	if err := r.selectThread(ctx, thread, false); err != nil {
		return err
	}
	if thread.UID == "" {
		return nil
	}
	if err := r.c.Delete(ctx, thread); err != nil {
		return err
	}
	*thread = *r.emptyThread(thread.Spec.AssistantName)
	return nil
}

func (r *run) deleteMessage(ctx context.Context, thread *v1.Thread) error {
	var (
		msgs     v1.MessageList
		filtered []v1.Message
	)
	err := r.c.List(ctx, &msgs, &kclient.ListOptions{
		Namespace: r.Namespace,
	})
	if err != nil {
		return err
	}

	for _, msg := range msgs.Items {
		if msg.Status.ThreadName == thread.Name && msg.Status.Message.Role == v1.RoleTypeUser {
			filtered = append(filtered, msg)
		}
	}

	var noErr ErrNoChoice
	msgName, err := selectItem(&msgs, "Select message")
	if errors.As(err, &noErr) {
		fmt.Println(err.Error())
		return nil
	} else if err != nil {
		return err
	}

	if thread.Spec.StartMessageName == msgName {
		if err := r.c.Delete(ctx, thread); err != nil {
			return err
		}
		*thread = *r.emptyThread(thread.Spec.AssistantName)
		return nil
	}

	return r.c.Delete(ctx, &v1.Message{
		ObjectMeta: metav1.ObjectMeta{
			Name:      msgName,
			Namespace: r.Namespace,
		},
	})
}

func (r *run) selectThread(ctx context.Context, thread *v1.Thread, printEmpty bool) error {
	var (
		threads  v1.ThreadList
		filtered []v1.Thread
	)
	err := r.c.List(ctx, &threads, &kclient.ListOptions{
		Namespace: r.Namespace,
	})
	if err != nil {
		return err
	}

	for _, t := range threads.Items {
		if t.Spec.AssistantName == thread.Spec.AssistantName {
			filtered = append(filtered, t)
		}
	}

	threads.Items = filtered

	if len(threads.Items) == 0 {
		if printEmpty {
			fmt.Println("No existing threads, creating new thread")
		}
		*thread = *r.emptyThread(thread.Spec.AssistantName)
		return nil
	}

	threadName, err := selectItem(&threads, "Choose a thread")
	if err != nil {
		return err
	}

	return r.c.Get(ctx, router.Key(thread.Namespace, threadName), thread)
}

func (r *run) nextMessage(ctx context.Context, thread *v1.Thread, msg *v1.Message) (string, error) {
	if msg != nil {
		if msg.Status.NextMessageName != "" {
			return msg.Status.NextMessageName, nil
		}
		if msg.Status.Message.Role != v1.RoleTypeAssistant {
			return "", fmt.Errorf("expecting assistant message for [%s] but got role [%s]", msg.Name, msg.Status.Message.Role)
		}
	}

	var content string
	for content == "" {
		err := survey.AskOne(&survey.Input{
			Message: r.getPrompt(thread),
			Help:    r.getHelp(),
		}, &content)
		if err != nil {
			return "", err
		}
	}

	switch content {
	case "dt":
		if err := r.deleteThread(ctx, thread); err != nil {
			return "", nil
		}
		return r.nextMessage(ctx, thread, nil)
	case "d":
		if err := r.deleteMessage(ctx, thread); err != nil {
			return "", err
		}
		if thread.Spec.StartMessageName != "" {
			return thread.Spec.StartMessageName, nil
		}
		return r.nextMessage(ctx, thread, nil)
	case "n":
		*thread = *r.emptyThread(thread.Spec.AssistantName)
		return r.nextMessage(ctx, thread, nil)
	case "r":
		if err := r.selectThread(ctx, thread, true); err != nil {
			return "", err
		}
		if thread.Spec.StartMessageName != "" {
			return thread.Spec.StartMessageName, nil
		}
		return r.nextMessage(ctx, thread, nil)
	case "q":
		os.Exit(0)
	case "a":
		r.Options.Assistant = ""
		r.Options.Thread = ""
		newThread, err := r.getThread(ctx)
		if err != nil {
			return "", err
		}
		*thread = *newThread
		return r.nextMessage(ctx, thread, nil)
	}

	var parentMessage string
	if msg != nil {
		parentMessage = msg.Name
	}

	msg = &v1.Message{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "m-",
			Namespace:    thread.Namespace,
		},
		Spec: v1.MessageSpec{
			Input: v1.MessageInput{
				Content: v1.Text(content),
			},
			ParentMessageName: parentMessage,
		},
		Status: v1.MessageStatus{},
	}

	if err := r.c.Create(ctx, msg); err != nil {
		return "", err
	}

	if thread.Spec.StartMessageName == "" {
		thread.Spec.StartMessageName = msg.Name
		if thread.UID == "" {
			if err := r.c.Create(ctx, thread); err != nil {
				return "", err
			}
		} else if err := r.c.Update(ctx, thread); err != nil {
			return "", err
		}
	}

	return msg.Name, nil
}

func (r *run) run(ctx context.Context) error {
	thread, err := r.getThread(ctx)
	if err != nil {
		return err
	}

	return r.printThread(ctx, thread)
}

func (r *run) printMessage(ctx context.Context, t *v1.Thread, name string) error {
	for {
		var (
			printed string
			w       = watcher.New[*v1.Message](r.c)
			msg     *v1.Message
			err     error
		)

		if name != "" {
			logrus.Debugf("Waiting on message %s", name)
			msg, err = w.ByName(ctx, r.Namespace, name, func(msg *v1.Message) (bool, error) {
				content := msg.Status.Message.String()

				if condition := meta.FindStatusCondition(msg.Status.Conditions, "Controller"); condition != nil && condition.Status == metav1.ConditionFalse &&
					condition.Message != "" {
					return false, errors.New(condition.Message)
				}

				if content == "" {
					return false, nil
				}

				fmt.Print(strings.TrimPrefix(content, printed))
				printed = content

				if msg.Status.InProgress {
					return false, nil
				}

				// It's done when it's an assistant message, or we have the next message
				if msg.Status.NextMessageName != "" || msg.Status.Message.Role == v1.RoleTypeAssistant {
					fmt.Println()
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				return err
			}
			fmt.Println()
		}

		name, err = r.nextMessage(ctx, t, msg)
		if err != nil {
			return err
		}
		if msg == nil {
			logrus.Debugf("Start message (thread %s) %s", t.Name, name)
		} else {
			logrus.Debugf("Next message (thread %s) %s->%s", t.Name, msg.Name, name)
		}
	}
}

func (r *run) printThread(ctx context.Context, t *v1.Thread) error {
	return r.printMessage(ctx, t, t.Spec.StartMessageName)
}

func (r *run) getAssistant(ctx context.Context) (string, error) {
	if r.Assistant != "" {
		return r.Assistant, nil
	}

	var assistants v1.AssistantList
	err := r.c.List(ctx, &assistants, &kclient.ListOptions{
		Namespace: r.Namespace,
	})
	if err != nil {
		return "", err
	}

	return selectItem(&assistants, "Choose an assistant")
}

func (r *run) emptyThread(assistant string) *v1.Thread {
	return &v1.Thread{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name.SafeConcatName(assistant, ""),
			Namespace:    r.Namespace,
		},
		Spec: v1.ThreadSpec{
			AssistantName: assistant,
		},
	}
}

func (r *run) getThread(ctx context.Context) (*v1.Thread, error) {
	var thread v1.Thread
	if r.Thread != "" {
		if err := r.c.Get(ctx, router.Key(r.Namespace, r.Thread), &thread); err != nil {
			return nil, err
		}
		return &thread, nil
	}

	assistant, err := r.getAssistant(ctx)
	if err != nil {
		return nil, err
	}

	return r.emptyThread(assistant), nil
}

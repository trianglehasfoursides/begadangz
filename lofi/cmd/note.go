package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/user"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/trianglehasfoursides/begadangz/lofi/util"
	"github.com/trianglehasfoursides/begadangz/validate"
	"github.com/zalando/go-keyring"
)

func init() {
	singleton := new(note)
	n := singleton.note()
	n.AddCommand(singleton.add())
	n.AddCommand(singleton.list())
	n.AddCommand(singleton.edit())
	n.AddCommand(singleton.remove())
	root.AddCommand(n)
}

type note struct{}

func (n *note) note() *cobra.Command {
	return &cobra.Command{
		Use:   "note",
		Short: "",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
}

func (n *note) add() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "",
		Run: func(cmd *cobra.Command, args []string) {
			var title string
			var content string
			// Edit form
			var confirmed bool
			form := huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title("Title").
					Value(&title).
					Validate(func(s string) error {
						if s == "" {
							return errors.New("Title is required")
						}
						return nil
					}),
				huh.NewText().
					Title("Content").
					Value(&content).
					Validate(func(s string) error {
						if s == "" {
							return errors.New("Content is required")
						}
						return nil
					}),
				huh.NewConfirm().
					Title("Submit?").
					Value(&confirmed),
			))

			form.Run()

			if !confirmed {
				fmt.Println("Cancelled.")
				return
			}

			usr, _ := user.Current()
			userid, err := keyring.Get("begadangz", usr.Name)
			if err != nil {
				util.Logger.Fatal(err.Error())
			}

			bearer := fmt.Sprintf("Bearer %s", strings.Split(userid, ":")[0])
			req := resty.New().NewRequest()
			r, err := req.
				SetHeader("Authorization", bearer).
				SetBody(map[string]string{
					"name":    title,
					"content": content,
				}).
				Post("http://localhost:8000/note")
			if err != nil {
				util.Logger.Fatal(err.Error())
			}

			body := r.Body()
			util.Error(r, body)

			fmt.Println("Succes")
		},
	}
}

func (n *note) list() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "",
		Run: func(cmd *cobra.Command, args []string) {
			usr, _ := user.Current()
			userid, err := keyring.Get("begadangz", usr.Name)
			if err != nil {
				util.Logger.Fatal(err.Error())
			}

			bearer := fmt.Sprintf("Bearer %s", strings.Split(userid, ":")[0])
			req := resty.New().NewRequest()
			r, err := req.
				SetHeader("Authorization", bearer).
				Get("http://localhost:8000/note")
			if err != nil {
				util.Logger.Fatal(err.Error())
			}

			body := r.Body()
			var notes []struct {
				Id      int    `json:"ID"`
				Title   string `json:"name"`
				Content string `json:"content"`
			}

			json.Unmarshal(body, &notes)
			items := make([]util.Item, len(notes))
			for i, n := range notes {
				items[i] = util.Item{Name: n.Title, Content: n.Content}
			}

			util.List(items)
		},
	}
}

func (n *note) edit() *cobra.Command {
	return &cobra.Command{
		Use:   "edit [note_name]",
		Short: "Edit an existing note by its name",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 || args[0] == "" {
				fmt.Println("Error: note name is required")
				return
			}

			req := resty.New().NewRequest()
			usr, _ := user.Current()
			userid, err := keyring.Get("begadangz", usr.Name)
			if err != nil {
				util.Logger.Fatal(err.Error())
			}

			bearer := fmt.Sprintf("Bearer %s", strings.Split(userid, ":")[0])
			url := fmt.Sprintf("http://localhost:8000/note/%s", args[0])

			// Fetch note from server
			res, err := req.SetHeader("Authorization", bearer).Get(url)
			if err != nil {
				util.Logger.Fatal(err.Error())
			}

			var note struct {
				Title   string `json:"title"`
				Content string `json:"content"`
			}
			body := res.Body()
			json.Unmarshal(body, &note)
			util.Error(res, body)

			// Edit form
			var confirmed bool
			form := huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title("Title").
					Value(&note.Title).
					Validate(func(s string) error {
						if s == "" {
							return errors.New("Title is required")
						}
						return nil
					}),
				huh.NewText().
					Title("Content").
					Value(&note.Content).
					Validate(func(s string) error {
						if s == "" {
							return errors.New("Content is required")
						}
						return nil
					}),
				huh.NewConfirm().
					Title("Submit?").
					Value(&confirmed),
			))

			form.Run()

			if !confirmed {
				fmt.Println("Cancelled.")
				return
			}

			// Send update
			res, err = req.
				SetHeader("Authorization", bearer).
				SetBody(map[string]string{
					"name":    note.Title,
					"content": note.Content,
				}).
				Put(url)
			if err != nil {
				util.Logger.Fatal(err.Error())
			}

			util.Error(res, res.Body())
			fmt.Println("Success")
		},
	}
}

func (n *note) remove() *cobra.Command {
	return &cobra.Command{
		Use:   "rm",
		Short: "",
		Run: func(cmd *cobra.Command, args []string) {
			if args == nil {
				return
			}
			name := args[0]
			if err := validate.Valid.Var(name, "required"); err != nil {
				util.Logger.Error(err.Error())
				return
			}

			usr, _ := user.Current()
			userid, err := keyring.Get("begadangz", usr.Name)
			if err != nil {
				util.Logger.Fatal(err.Error())
			}

			bearer := fmt.Sprintf("Bearer %s", strings.Split(userid, ":")[0])
			url := fmt.Sprintf("http://localhost:8000/note/%s", name)
			req := resty.New().NewRequest()
			r, err := req.
				SetHeader("Authorization", bearer).
				Delete(url)
			if err != nil {
				util.Logger.Fatal(err.Error())
			}

			body := r.Body()
			util.Error(r, body)
			util.Logger.Info("Succes")
		},
	}
}

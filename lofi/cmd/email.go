package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/trianglehasfoursides/begadangz/lofi/util"
	"github.com/trianglehasfoursides/begadangz/validate"
	"github.com/zalando/go-keyring"
)

// func init() {
// 	email.AddCommand(send)
// 	email.AddCommand(history)
// 	email.AddCommand(view)
// 	email.AddCommand(drop)
// 	root.AddCommand(email)
// }

type mail struct {
	From       string `json:"name"`
	To         string `json:"to"`
	Subject    string `json:"subject"`
	Mail       string `json:"mail"`
	Attachment string
}

var email = &cobra.Command{
	Use:   "email",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var send = &cobra.Command{
	Use:   "send",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		msg := new(mail)

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("From").
					Validate(func(s string) error {
						if err := validate.Valid.Var(s, "required"); err != nil {
							return err
						}
						return nil
					}).
					Value(&msg.From),
				huh.NewInput().
					Title("To").
					Validate(func(s string) error {
						if err := validate.Valid.Var(s, "required"); err != nil {
							return err
						}
						return nil
					}).
					Value(&msg.To),
				huh.NewInput().
					Title("Subject").
					Validate(func(s string) error {
						if err := validate.Valid.Var(s, "required"); err != nil {
							return err
						}
						return nil
					}).
					Value(&msg.Subject),
				huh.NewText().Title("Mail").
					Description("we support markdown").
					Validate(func(s string) error {
						if err := validate.Valid.Var(s, "required"); err != nil {
							return err
						}
						return nil
					}).
					Value(&msg.Mail),
				huh.NewFilePicker().
					Value(&msg.Attachment).
					Validate(func(s string) error {
						if s == "" {
							return nil
						}
						info, _ := os.Stat(s)
						if info.Size() > 10*1024*1024 {
							return errors.New("The uploaded file is too large. Maximum size is 10MB.")
						}
						return nil
					}).
					ShowPermissions(true).
					ShowHidden(true).
					FileAllowed(true).
					ShowSize(true).
					WithAccessible(true),
				huh.NewConfirm().
					Title("submit"),
			),
		)

		if err := form.Run(); err != nil {
			util.Logger.Fatal(err.Error())
		}

		req := resty.New().NewRequest()

		if msg.Attachment != "" {
			req.SetFile("attachment", msg.Attachment)
		}

		usr, _ := user.Current()
		userid, err := keyring.Get("begadangz", usr.Name)
		if err != nil {
			util.Logger.Fatal(err.Error())
		}

		bearer := fmt.Sprintf("Bearer %s", strings.Split(userid, ":")[0])
		r, err := req.
			SetHeader("Authorization", bearer).
			SetMultipartFormData(map[string]string{
				"from":    msg.From,
				"to":      msg.To,
				"subject": msg.Subject,
				"mail":    msg.Mail,
			}).
			Post("http://localhost:8000/email/send")

		if err != nil {
			util.Logger.Fatal(err.Error())
		}
		util.Error(r, r.Body())
	},
}

var history = &cobra.Command{
	Use:   "history",
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
			Get("http://localhost:8000/email/history")
		if err != nil {
			util.Logger.Fatal(err.Error())
		}

		body := r.Body()

		var emails []struct {
			Id         int    `json:"ID"`
			From       string `json:"from"`
			To         string `json:"to"`
			Mail       string `json:"mail"`
			Subject    string `json:"subject"`
			Attachment string `json:"attachment"`
		}

		util.Error(r, body)
		json.Unmarshal(body, &emails)
		rows := make([]table.Row, len(emails))
		for i, e := range emails {
			rows[i] = table.Row{strconv.Itoa(e.Id), e.From, e.To, e.Subject, e.Mail, e.Attachment}
		}

		util.RenderTable(rows...)
	},
}

var view = &cobra.Command{
	Use:   "view",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		if args == nil {
			return
		}
		id := args[0]
		if err := validate.Valid.Var(id, "required"); err != nil {
			util.Logger.Error(err.Error())
			return
		}

		usr, _ := user.Current()
		userid, err := keyring.Get("begadangz", usr.Name)
		if err != nil {
			util.Logger.Fatal(err.Error())
		}

		bearer := fmt.Sprintf("Bearer %s", strings.Split(userid, ":")[0])
		url := fmt.Sprintf("http://localhost:8000/email/%s", id)
		req := resty.New().NewRequest()
		r, err := req.
			SetHeader("Authorization", bearer).
			Get(url)
		if err != nil {
			util.Logger.Fatal(err.Error())
		}

		body := r.Body()
		util.Error(r, body)

		var email struct {
			Id         int    `json:"ID"`
			From       string `json:"from"`
			To         string `json:"to"`
			Mail       string `json:"mail"`
			Subject    string `json:"subject"`
			Attachment string `json:"attachment"`
		}

		json.Unmarshal(body, &email)
		content := fmt.Sprintf(`
Id : %d

From : %s

To : %s

Subject : %s

Attachment : %s

Mail

%s`,
			email.Id,
			email.From,
			email.To,
			email.Subject,
			email.Attachment,
			email.Mail)

		out, err := glamour.Render(content, "dark")
		if err != nil {
			util.Logger.Error(err.Error())
			return
		}
		fmt.Print(out)
	},
}

var drop = &cobra.Command{
	Use:   "rm",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		if args == nil {
			return
		}
		id := args[0]
		if err := validate.Valid.Var(id, "required"); err != nil {
			util.Logger.Error(err.Error())
			return
		}

		usr, _ := user.Current()
		userid, err := keyring.Get("begadangz", usr.Name)
		if err != nil {
			util.Logger.Fatal(err.Error())
		}

		bearer := fmt.Sprintf("Bearer %s", strings.Split(userid, ":")[0])
		url := fmt.Sprintf("http://localhost:8000/email/%s", id)
		req := resty.New().NewRequest()
		r, err := req.
			SetHeader("Authorization", bearer).
			Delete(url)
		if err != nil {
			util.Logger.Fatal(err.Error())
		}

		body := r.Body()
		util.Error(r, body)
		log.Info("Succes")
	},
}

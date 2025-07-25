package cmd

import (
	"fmt"
	"net/http"
	"os/user"

	"github.com/gin-gonic/gin"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

func init() {
	login.AddCommand(google)
	login.AddCommand(github)
	auth.AddCommand(login)
	auth.AddCommand(logout)
	root.AddCommand(auth)
}

var auth = &cobra.Command{
	Use:   "auth",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var login = &cobra.Command{
	Use:   "login",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var google = &cobra.Command{
	Use:   "google",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		var srv *http.Server
		browser.OpenURL("http://localhost:8000/auth/google")
		router := gin.Default()
		router.GET("/auth/callback", func(ctx *gin.Context) {
			token, isset := ctx.GetQuery("token")
			if isset == false {
				srv.Shutdown(ctx)
				return
			}
			email, isset := ctx.GetQuery("email")
			if isset == false {
				srv.Shutdown(ctx)
				return
			}
			usr, err := user.Current()
			if err != nil {
				srv.Shutdown(ctx)
				return
			}
			secret := fmt.Sprintf("%s:%s", token, email)
			keyring.Set("begadangz", usr.Name, secret)
			fmt.Println(usr.Name, secret)
			ctx.Writer.Write([]byte(`sucesss`))
			ctx.Writer.Flush()
			srv.Shutdown(ctx)
		})

		srv = &http.Server{
			Addr:    "localhost:9000",
			Handler: router,
		}

		srv.ListenAndServe()
	},
}

var github = &cobra.Command{
	Use:   "github",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		var srv *http.Server
		browser.OpenURL("http://localhost:8000/auth/github")
		router := gin.Default()
		router.GET("/auth/callback", func(ctx *gin.Context) {
			token, isset := ctx.GetQuery("token")
			if isset == false {
				srv.Shutdown(ctx)
				return
			}
			email, isset := ctx.GetQuery("email")
			if isset == false {
				srv.Shutdown(ctx)
				return
			}
			usr, err := user.Current()
			if err != nil {
				srv.Shutdown(ctx)
				return
			}
			secret := fmt.Sprintf("%s:%s", token, email)
			keyring.Set("begadangz", usr.Name, secret)
			fmt.Println(usr.Name, secret)
			ctx.Writer.Write([]byte(`sucesss`))
			ctx.Writer.Flush()
			srv.Shutdown(ctx)
		})

		srv = &http.Server{
			Addr:    "localhost:9000",
			Handler: router,
		}

		srv.ListenAndServe()
	},
}

var logout = &cobra.Command{
	Use:   "logout",
	Short: "",
	Run: func(cmd *cobra.Command, args []string) {
		usr, _ := user.Current()
		keyring.Delete("begadangz", usr.Name)
	},
}

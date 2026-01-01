package main

import (
	"log"

	"github.com/pocketbase/pocketbase"
)

func main() {
	app := pocketbase.New()

    /*
    // Serve static files from pb_public
    app.OnServe().BindFunc(func(e *core.ServeEvent) error {
        e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS("./pb_public"), false))
        return nil
    })
    */

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

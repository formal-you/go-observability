
package main

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	// Local console logging only.
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// No OTLP exporter / remote endpoint is configured.
	// Use a no-op tracer so tracing produces no remote traffic.
	tracer := otel.Tracer("local-only")

	ctx := context.Background()

	_, span := tracer.Start(ctx, "application.start")
	defer span.End()

	slog.Info("Application started",
		"mode", "local-only",
	)
}

var _ trace.Tracer = otel.Tracer("local-only")
import tkinter as tk
from PIL import Image, ImageTk

root = tk.Tk()
root.attributes("-fullscreen", True)

# မင်းပေးထားတဲ့ပုံ
image = Image.open("/mnt/data/2781.jpg")

# Screen အရွယ်အစားနဲ့ ကိုက်အောင်ပြောင်း
screen_w = root.winfo_screenwidth()
screen_h = root.winfo_screenheight()

image = image.resize((screen_w, screen_h))
bg_image = ImageTk.PhotoImage(image)

# နောက်ခံပုံ
background = tk.Label(root, image=bg_image)
background.place(x=0, y=0, relwidth=1, relheight=1)

# အပေါ်က စာသား
label = tk.Label(
    root,
    text="FUCK YOU",
    font=("Arial", 60, "bold"),
    fg="red",
    bg="black"
)
label.place(relx=0.5, rely=0.5, anchor="center")

# Esc နှိပ်ရင် ပိတ်
root.bind("<Escape>", lambda event: root.destroy())

root.mainloop()

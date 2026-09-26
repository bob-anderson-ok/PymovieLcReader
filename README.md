# PymovieLcReader
The application reads and displays the aperture file recorded automatically by PyMovie 4.2.3 and above.

PyMovie writes a `<csv name>.pymovie` file alongside each CSV it saves: one
record per aperture per frame, holding the light curve values and the
aperture's image and sampling mask. Files from PyMovie 4.2.3 and later also
hold the first frame analysed, with the apertures drawn on it. PymovieLcReader
opens these files and shows:

- the light curves of the apertures you select, as intensity or appsum, with
  per-curve colors, adjustable limits and frame range, and a cursor that steps
  through the points with the arrow keys;
- a window per selected aperture with its raw image and the data inside its
  sampling mask at the cursor frame, black/white level sliders, pixels at or
  above the saturation level in red, and the value of the pixel under the
  mouse pointer;
- the initial frame with the colored aperture boxes and a table of the
  apertures' positions.

The file format is described in [aperture-record-format.md](aperture-record-format.md).

## Building

PymovieLcReader is written in Go and uses the [Fyne](https://fyne.io) GUI
toolkit, which needs cgo and a C compiler.

### Requirements

- **Go 1.25** or later.
- **A C compiler** for cgo, as Fyne requires:
  - **Windows:** a 64-bit gcc, e.g. from [MSYS2](https://www.msys2.org)
    (`pacman -S mingw-w64-x86_64-gcc`). Put `C:\msys64\mingw64\bin` on your
    `PATH` **ahead of** any older 32-bit MinGW, or the cgo step fails.
  - **macOS:** the Xcode command line tools (`xcode-select --install`).
  - **Linux:** gcc and the X11/OpenGL development packages, e.g. on Debian or
    Ubuntu: `sudo apt install gcc libgl1-mesa-dev xorg-dev`.

  See Fyne's [getting started](https://docs.fyne.io/started/) page for details.

The app has been built and used on Windows. It should build on macOS and Linux
too, but hasn't been tried there; window positions are only remembered on Windows.

### Build and run

```sh
git clone https://github.com/bob-anderson-ok/PymovieLcReader.git
cd PymovieLcReader
go build -o PymovieLcReader.exe .
./PymovieLcReader.exe                  # or: ./PymovieLcReader.exe path/to/file.pymovie
```

The first build takes a few minutes while cgo compiles Fyne's graphics
libraries; later builds are quick. On macOS and Linux, drop the `.exe`.

The app opens a `.pymovie` file named on the command line, otherwise the last
file it opened, otherwise `LC-test.pymovie` if that is in the working folder.
Use **Open .pymovie file...** to pick another.

### Release build (Windows)

```sh
go build -trimpath -ldflags "-H windowsgui -s -w" -o dist/PymovieLcReader.exe .
```

`-H windowsgui` stops a console window opening with the app; `-s -w` leave out
debug information; `-trimpath` keeps local folder paths out of the exe. The exe
is self-contained: it needs only standard Windows DLLs (and an OpenGL-capable
graphics driver). `dist/` is ignored by git.

The version shown in the title bar is `appVersion` in `main.go`. Releases are
tagged `v1.0`, `v1.1`, ...

### Tests

```sh
go test ./...                    # the app
cd go-reader && go test ./...    # the .pymovie reader
```

The app's tests use `LC-test.pymovie` (written before PyMovie 4.2.3, so without an
initial frame) and the files in `go-reader/testdata/`.

## Layout

| Path | Contents |
|---|---|
| `main.go` | Startup, the main window, the Open dialog, the version |
| `viewer.go` | The controls and plot for one open file |
| `curves.go` | Light curves built from the records; limits, frames, timestamps |
| `plot.go`, `plotwidget.go` | Drawing the plot (with [gonum/plot](https://github.com/gonum/plot)), the cursor and clicks |
| `imagewindow.go`, `pixelview.go` | The aperture image windows and the pixel under the pointer |
| `initialframe.go` | The initial frame window |
| `winpos*.go` | Remembering window sizes and (on Windows) positions |
| `go-reader/` | Package `pymoviefile`, which reads `.pymovie` files |
| `aperture-record-format.md` | The file format |
| `LC-test.pymovie` | A sample file, used by the tests |

`go-reader/` and `aperture-record-format.md` are copies of the ones in the
[PyMovie repository](https://github.com/bob-anderson-ok/pymovie), which owns
the format; update them from there when the format changes.
`go-reader/testdata/make_testdata.py` regenerates the test files, and must be
run from the PyMovie repository, since it uses PyMovie's own writer.

Settings such as the dot size, the last file and window positions are kept in
Fyne's preferences for the app ID `com.pymovie.lcreader` (on Windows, in
`%APPDATA%\fyne\com.pymovie.lcreader\preferences.json`).

## License

MIT: see [LICENSE](LICENSE). You may use, copy, modify and distribute the code, provided the copyright and permission notice are kept.

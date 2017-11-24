package main

import (
	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"github.com/faiface/pixel/text"
	"golang.org/x/image/colornames"
	"unicode"
	"golang.org/x/image/font/basicfont"
	"fmt"
	"net"
	"encoding/json"
	"github.com/faiface/pixel/imdraw"
	"time"
	"math"
	"github.com/snorristurluson/exsim_commands"
)

type SolarsystemViewer struct {
	state *exsim_commands.State
}

func NewSolarsystemViewer() (*SolarsystemViewer){
	return &SolarsystemViewer{
		state: exsim_commands.NewState(),
	}
}

func (viewer* SolarsystemViewer) render(imd* imdraw.IMDraw, atlas* text.Atlas, thickness float64) ([]*text.Text) {
	labels := []*text.Text{}
	shipsById := make(map[int64]exsim_commands.ShipData)
	for _, ship := range(viewer.state.Ships) {
		shipsById[ship.Owner] = ship
	}

	for _, ship := range(viewer.state.Ships) {
		x := ship.Position.X
		y := ship.Position.Y
		pos := pixel.V(x,y)

		imd.Color = colornames.Black
		imd.Push(pos)
		imd.Circle(10, thickness)

		if false {
			imd.Color = colornames.Gray
			imd.Push(pos)
			imd.Circle(100, thickness)

		}
		for _, shipInRange := range(ship.InRange) {
			other, found := shipsById[shipInRange]
			if found {
				otherPos := pixel.V(other.Position.X, other.Position.Y)
				imd.Color = colornames.Gray
				imd.Push(pos, otherPos)
				imd.Line(thickness)
			}
		}

		label := text.New(pos, atlas)
		label.Color = colornames.Black
		fmt.Fprintf( label,"%v", ship.Owner)
		labels = append(labels, label)
	}

	return labels
}

type Client struct {
	userid int64
	connection net.Conn
	targetLocation exsim_commands.Vector3
}

func NewClient(userid int64) (*Client) {
	return &Client{
		userid: userid,
	}
}

func (client *Client) Connect(address string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Couldn't connect to %v", address)
		return
	}
	fmt.Printf("Connection established\n")
	client.connection = conn

	msg := fmt.Sprintf(`{"user": %d}` + "\n", client.userid)
	client.connection.Write([]byte(msg))
}

func (client *Client) SetTargetLocation(loc exsim_commands.Vector3) {
	msg := fmt.Sprintf(
		`{"command": "settargetlocation", "params": {"location": {"x": %v, "y": %v, "z": %v}}}` + "\n",
		loc.X, loc.Y, loc.Z)
	client.connection.Write([]byte(msg))

}

func (client *Client) ReceiveLoop(cmdQueue chan *exsim_commands.State) {
	decoder := json.NewDecoder(client.connection)
	for {
		var cmd exsim_commands.State
		err := decoder.Decode(&cmd)
		if err != nil {
			fmt.Printf("Error in Decode: %v\n", err)
			break
		}
		cmdQueue <- &cmd
	}
}


func run() {
	client := NewClient(1000)
	client.Connect(":4040")

	cfg := pixelgl.WindowConfig{
		Title:  "exsim client",
		Bounds: pixel.R(0, 0, 800, 600),
		VSync:  true,
		Resizable: true,
	}
	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}

	viewer := NewSolarsystemViewer()
	camPos := pixel.ZV
	camSpeed := 500.0
	camZoom := 1.0
	camZoomSpeed := 1.2

	imd := imdraw.New(nil)
	labels := []*text.Text{}

	atlas := text.NewAtlas(basicfont.Face7x13, text.ASCII, text.RangeTable(unicode.Latin))
	numShips := text.New(pixel.V(10, 10), atlas)
	numShips.Color = colornames.Darkgreen
	fmt.Fprintf( numShips, "Ships: %v", len(viewer.state.Ships))

	recvChannel := make(chan *exsim_commands.State)
	go client.ReceiveLoop(recvChannel)
	last := time.Now()
	for !win.Closed() {
		dt := time.Since(last).Seconds()
		last = time.Now()

		select {
		case stateReceived := <- recvChannel:
			viewer.state = stateReceived
			numShips.Clear()
			numShips.Dot = numShips.Orig
			fmt.Fprintf( numShips, "Ships: %v", len(viewer.state.Ships))
			imd.Clear()
			labels = viewer.render(imd, atlas, 1.0 / camZoom)
		default:
			// No data received
		}
		win.Clear(colornames.Skyblue)

		if win.Pressed(pixelgl.KeySpace) {
			fmt.Println("Resetting camera")
			pos := viewer.state.Ships["ship_1000"].Position
			camPos.X = pos.X
			camPos.Y = pos.Y
			fmt.Printf("%v, %v\n", pos.X, pos.Y)
		}

		if win.Pressed(pixelgl.KeyLeft) {
			camPos.X -= camSpeed * dt
		}
		if win.Pressed(pixelgl.KeyRight) {
			camPos.X += camSpeed * dt
		}
		if win.Pressed(pixelgl.KeyDown) {
			camPos.Y -= camSpeed * dt
		}
		if win.Pressed(pixelgl.KeyUp) {
			camPos.Y += camSpeed * dt
		}
		camZoom *= math.Pow(camZoomSpeed, win.MouseScroll().Y)

		cam := pixel.IM.Scaled(camPos, camZoom).Moved(win.Bounds().Center().Sub(camPos))
		win.SetMatrix(cam)

		if win.JustPressed(pixelgl.MouseButtonLeft) {
			mousePos := cam.Unproject(win.MousePosition())
			pos := exsim_commands.Vector3{X:mousePos.X, Y:mousePos.Y}
			client.SetTargetLocation(pos)
		}
		imd.Draw(win)
		for _, label := range(labels) {
			label.Draw(win, pixel.IM)
		}

		win.SetMatrix(pixel.IM)
		numShips.Draw(win, pixel.IM)

		win.Update()
	}
}

func main() {
	pixelgl.Run(run)
}
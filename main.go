package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"

	"github.com/ebitengine/oto/v3"
	"github.com/ezmidi/go-meltysynth/meltysynth"
	"github.com/mattrtaylor/go-rtmidi"
)

// AudioReader generates audio samples from the synthesizer.
type AudioReader struct {
	synthesizer *meltysynth.Synthesizer
}

// Read fills the provided byte slice with audio data.
func (ar *AudioReader) Read(p []byte) (n int, err error) {
	left := make([]float32, 2)
	right := make([]float32, 2)

	// Render the waveform
	ar.synthesizer.Render(left, right)

	// Prepare audio data for output as float32 values
	leftSample := left[0]   // Channel 1
	rightSample := right[0] // Channel 2

	// Convert float32 samples to bytes (little-endian)
	leftBytes := make([]byte, 4)
	rightBytes := make([]byte, 4)

	binary.LittleEndian.PutUint32(leftBytes, math.Float32bits(leftSample))
	binary.LittleEndian.PutUint32(rightBytes, math.Float32bits(rightSample))

	// Write left channel float32 (4 bytes) to the buffer
	copy(p[n:], leftBytes)
	n += 4

	// Write right channel float32 (4 bytes) to the buffer
	copy(p[n:], rightBytes)
	n += 4

	return n, nil
}

// Seek sets the current position in the audio stream.
func (ar *AudioReader) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

// handleMidiMessage processes incoming MIDI messages
func handleMidiMessage(msg []byte, synthesizer *meltysynth.Synthesizer) {
	if len(msg) > 0 {
		fmt.Printf("MIDI Message: %v\n", msg) // Log MIDI messages
		switch msg[0] & 0xF0 {
		case 0x90: // Note On
			if len(msg) < 3 {
				return
			}
			note := msg[1]
			velocity := msg[2]
			if velocity > 0 {
				synthesizer.NoteOn(0, int32(note), int32(velocity))
			} else {
				synthesizer.NoteOff(0, int32(note))
			}
		case 0x80: // Note Off
			if len(msg) < 3 {
				return
			}
			note := msg[1]
			synthesizer.NoteOff(0, int32(note))
		}
	}
}

// main function
func main() {
	// Load the sound font
	sf2, err := os.Open("Mergedsoundfont.sf2")
	if err != nil {
		log.Fatalf("Failed to open sound font: %v", err)
	}
	soundFont, err := meltysynth.NewSoundFont(sf2)
	if err != nil {
		log.Fatalf("Failed to create sound font: %v", err)
	}
	sf2.Close()

	// Create the synthesizer.
	settings := &meltysynth.SynthesizerSettings{
		SampleRate:            48000,
		BlockSize:             512,   // 例: デフォルトのブロックサイズ
		MaximumPolyphony:      500,   // 例: デフォルトのポリフォニー
		EnableReverbAndChorus: false, // 例: リバーブとコーラスを有効にする
	}

	synthesizer, err := meltysynth.NewSynthesizer(soundFont, settings)
	if err != nil {
		log.Fatalf("Failed to create synthesizer: %v", err)
	}

	// Set up MIDI input
	midiIn, err := rtmidi.NewMIDIInDefault()
	if err != nil {
		log.Fatalf("Failed to create MIDI input: %v", err)
	}
	defer midiIn.Close()

	// Get the count of available MIDI input devices
	portCount, err := midiIn.PortCount()
	if err != nil {
		log.Fatalf("Failed to get port count: %v", err)
	}

	if portCount == 0 {
		log.Fatalf("No MIDI input devices found.")
	}

	fmt.Println("Available MIDI Input Devices:")
	for i := 0; i < portCount; i++ {
		deviceName, err := midiIn.PortName(i)
		if err != nil {
			log.Fatalf("Failed to get port name: %v", err)
		}
		fmt.Printf("%d: %s\n", i, deviceName)
	}

	// Choose a device to open (adjust index based on available devices)
	portIndex := 0 // Change this index if needed
	if portIndex < portCount {
		err = midiIn.OpenPort(portIndex, "")
		if err != nil {
			log.Fatalf("Failed to open MIDI port: %v", err)
		}
	} else {
		log.Fatalf("Invalid port index: %d", portIndex)
	}

	// Set the callback function for MIDI input
	err = midiIn.SetCallback(func(midiIn rtmidi.MIDIIn, msg []byte, deltaTime float64) {
		handleMidiMessage(msg, synthesizer)
	})
	if err != nil {
		log.Fatalf("Failed to set MIDI callback: %v", err)
	}

	// Initialize Oto for audio playback
	options := oto.NewContextOptions{
		SampleRate:   48000,
		ChannelCount: 2,
		Format:       oto.FormatFloat32LE, // Change this to int16 for 16-bit output
	}

	context, ready, err := oto.NewContext(&options)
	if err != nil {
		log.Fatalf("Failed to create audio context: %v", err)
	}

	// Wait for the context to be ready
	<-ready

	// Create an instance of the audio reader
	audioReader := &AudioReader{synthesizer: synthesizer}

	// Create a new player that will read from the AudioReader
	player := context.NewPlayer(audioReader)
	if player == nil {
		log.Fatal("Failed to create player")
	}

	// Play starts playing the sound and returns without waiting for it (Play() is async).
	player.Play()

	// Keep the program running
	select {}
}

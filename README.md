# accbroadcastingsdk

SDK for using the broadcasting interface of Assetto Corsa Competizione (in Go)

This SDK resembles as much as possible the C# SDK that was published by Kunos Simulazione
and which can be found in the `steamapps/common/Assetto Corsa Competizione/sdk` folder.

Following the Go philosophy, documentation can be found in the code (and extracted using godoc).
Thus details about the interpretation of the data in the ACC Broadcasting interface is mainly
in [buffer.go](https://github.com/toonknapen/accbroadcastingsdk/blob/master/network/buffer.go#L104)
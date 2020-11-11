package network

const Code = "code"

// The address provided for the UDP interface did not resolve and thus client will stop
const ErrorAddressNotResolved = 1

// self-explanatory. Client will stop.
const ErrorSetupUDPConnection = 2

// The reading from the ACC UDP interface time'd out.
const ErrorReadTimeout = 3

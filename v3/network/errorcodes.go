package network

const Code = "code"

// The address provided for the UDP interface did not resolve and thus client will stop
const ErrorAddressNotResolved = 1001

// self-explanatory. Client will stop.
const ErrorSetupUDPConnection = 1002

// The reading from the ACC UDP interface time'd out.
const ErrorReadTimeout = 1003

// After the UDP connection is set up, the request to receive information from ACC
// is send to ACC
const InfoRegistrationReqSendToAcc = 1004

// ACC acknowledged the registration and has returned a connection-id
const InfoRegistrationAckByAcc = 1005

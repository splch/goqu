// Package braket implements a [backend.Backend] for Amazon Braket quantum
// cloud.
//
// This is a separate Go module to isolate the AWS SDK dependency. [New]
// creates a backend from an [aws.Config]. Use [WithDevice] for short device
// names (sv1, ionq.forte, iqm.garnet, rigetti.ankaa) or [WithDeviceARN]
// for explicit ARNs. Results are retrieved from S3.
//
// [DeviceARN] resolves a short name to its full ARN; [DeviceTarget] returns
// the corresponding [target.Target].
package braket

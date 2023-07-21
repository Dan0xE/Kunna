# Kunna - GitLab to BunnyCDN Synchronization

Kunna is a Go-based synchronization tool that mirrors our projects main branch (regarded as the production branch) from our GitLab instance to our BunnyCDN instance. This makes our projects available for download via our installer.

## Getting Started

Clone this repository to any location on your computer.

```bash
git clone https://github.com/aeronautical-studios/kunna.git
```

## Prerequisites

You will need Go installed on your computer to build and run the application. The Go version used for development is Go ``1.19.3``.

## Configuration

Kunna uses a ``config.json`` file for its configuration. Here is an example:

```json
{
  "DiscordWebHook": "",
  "TempStoragePath": "",
  "GitlabInstanceUrl": "",
  "GitLabAPIKey": "",
  "BunnyCDNStorageUrl": "",
  "BunnyCDNStoragePullZone": "",
  "BunnyCDNAPIKey": "",
  "BunnyCDNApiUrl": ""
}
```

**Note:** Please replace the empty strings with your actual values.

## How to run Kunna

To run Kunna, use the command:

```bash
go run .
```

To build an executable, use the command:

```bash
go build
```

## Error logging

Kunna includes a logging system that outputs to a file. Log files are named ``log_<timestamp>.log.`` If an error occurs, an embed will be sent to a Discord channel via a webhook (url specified in the config.json).\

## Temporary Storage

Kunna uses temporary storage to process files before sending them to the BunnyCDN. The location of this storage is specified in the ``config.json`` file.

## File Comparison

File comparison is used to decide which files need to be uploaded or deleted from BunnyCDN. The decision is based on comparing hashes of files (generated by kushn) from GitLab and BunnyCDN.

## Synchronization

Kunna will automatically sync repositories at intervals. The sync operation consists of fetching repositories from GitLab, comparing files, and performing necessary upload or delete operations on BunnyCDN.

## Reporting Bugs

If you encounter any bugs or issues, feel free to open an issue in this repository.

## Contributing

We welcome any form of contribution. Please first discuss the change you wish to make via an issue.

## License

Please see the ``LICENSE`` file for details on the license.

## Contact

For any inquiries or support, please contact us via dev@aeronauticalstudios.com.

## Acknowledgments

- The Go team for their wonderful programming language
- The GitLab team for their awesome platform
- The BunnyCDN team for their robust CDN service
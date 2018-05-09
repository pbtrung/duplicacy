# Duplicacy: A lock-free deduplication cloud backup tool

The following features are added to this fork:
* Encryption: Threefish with 1024 bit key
* Password hashing: Argon2
* Compression: zstd

Duplicacy is a new generation cross-platform cloud backup tool based on the idea of [Lock-Free Deduplication](https://github.com/gilbertchen/duplicacy/wiki/Lock-Free-Deduplication).  It is the only cloud backup tool that allows multiple computers to back up to the same storage simultaneously without using any locks (thus readily amenable to various cloud storage services).

## Features

Duplicacy currently supports major cloud storage providers (Amazon S3, Google Cloud Storage, Microsoft Azure, Dropbox, Backblaze B2, Google Drive, Microsoft OneDrive, Hubic) and offers all essential features of a modern backup tool:

* Duplicacy is the *only* cloud backup tool that allows multiple computers to back up to the same cloud storage, taking advantage of cross-computer deduplication whenever possible, without direct communication among them.  This feature literally turns any cloud storage server supporting only a basic set of file operations into a sophisticated deduplication-aware server.  

* Unlike other chunk-based backup tools where chunks are grouped into pack files and a chunk database is used to track which chunks are stored inside each pack file, Duplicacy takes a database-less approach where every chunk is saved independently using its hash as the file name to facilitate quick lookups.  The lack of a centralized chunk database not only makes the implementation less error-prone, but also produces a highly maintainable piece of software with plenty of room for development of new features and usability enhancements.

* Duplicacy is fast.  While the performance wasn't the top-priority design goal, Duplicacy has been shown to outperform other backup tools by a considerable margin, as indicated by the following results obtained from a [benchmarking experiment](https://github.com/gilbertchen/benchmarking) backing up the [Linux code base](https://github.com/torvalds/linux) using Duplicacy and 3 other open-source backup tools.

[![Comparison of Duplicacy, restic, Attic, duplicity](https://github.com/gilbertchen/duplicacy/blob/master/images/duplicacy_benchmark_speed.png "Comparison of Duplicacy, restic, Attic, duplicity")](https://github.com/gilbertchen/benchmarking)

## Getting Started

* [A brief introduction](https://github.com/gilbertchen/duplicacy/wiki/Quick-Start)
* [Command references](https://github.com/gilbertchen/duplicacy/wiki)
* [Building from source](https://github.com/gilbertchen/duplicacy/wiki/Installation)

## Storages

With Duplicacy, you can back up files to local or networked drives, SFTP servers, or many cloud storage providers.  The following storages are supported by Duplicacy:

* Amazon S3
* Backblaze B2
* DigitalOcean Spaces
* Dropbox
* Google Cloud Storage
* Google Drive
* Hubic
* Microsoft Azure
* Microsoft OneDrive
* SFTP
* Wasabi

Please consult the [wiki page](https://github.com/gilbertchen/duplicacy/wiki/Storage-Backends) on how to set up Duplicacy to work with each cloud storage.

## License

* Free for personal use or commercial trial
* Non-trial commercial use requires per-user CLI licenses available from [duplicacy.com](https://duplicacy.com/buy) at a cost of $20 per year
* A user is defined as the computer account that creates or edits the files to be backed up; if a backup contains files created or edited by multiple users for commercial purposes, one CLI license is required for each user
* The computer with a valid commercial license for the GUI version may run the CLI version without a CLI license
* CLI licenses are not required to restore or manage backups; only the backup command requires valid CLI licenses
* Modification and redistribution are permitted, but commercial use of derivative works is subject to the same requirements of this license

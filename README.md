# Duplicacy: A lock-free deduplication cloud backup tool

Duplicacy is a new generation cross-platform cloud backup tool based on the idea of [Lock-Free Deduplication](https://github.com/gilbertchen/duplicacy/wiki/Lock-Free-Deduplication).  It is the only cloud backup tool that allows multiple computers to back up to the same storage simultaneously without using any locks (thus readily amenable to various cloud storage services).

## Features

Duplicacy currently supports major cloud storage providers (Amazon S3, Google Cloud Storage, Microsoft Azure, Dropbox, Backblaze B2, Google Drive, Microsoft OneDrive, Hubic, and Sia) and offers all essential features of a modern backup tool:

* Incremental backup: only back up what has been changed
* Full snapshot: although each backup is incremental, it must behave like a full snapshot for easy restore and deletion
* Deduplication: identical files must be stored as one copy (file-level deduplication), and identical parts from different files must be stored as one copy (block-level deduplication)
* Encryption: encrypt not only file contents but also file paths, sizes, times, etc.
* Deletion: every backup can be deleted independently without affecting others
* Concurrent access: multiple clients can back up to the same storage at the same time
* Snapshot migration: all or selected snapshots can be migrated from one storage to another

The following features are added to this fork:
* Encryption: Threefish with 1024 bit key
* Password hashing: Argon2
* Compression: zstd
* **[Sia](https://sia.tech)** support: a decentralized cloud storage platform that uses a blockchain to facilitate payments

The key idea of **[Lock-Free Deduplication](https://github.com/gilbertchen/duplicacy/wiki/Lock-Free-Deduplication)** can be summarized as follows:

* Use variable-size chunking algorithm to split files into chunks
* Store each chunk in the storage using a file name derived from its hash, and rely on the file system API to manage chunks without using a centralized indexing database
* Apply a *two-step fossil collection* algorithm to remove chunks that become unreferenced after a backup is deleted

## Getting Started

* [A brief introduction](https://github.com/gilbertchen/duplicacy/wiki/Quick-Start)
* [Command references](https://github.com/gilbertchen/duplicacy/wiki)

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
* **[Sia](https://sia.tech)**
* Wasabi

Please consult the [wiki page](https://github.com/gilbertchen/duplicacy/wiki/Storage-Backends) on how to set up Duplicacy to work with each cloud storage.

## License

* Free for personal use or commercial trial
* Non-trial commercial use requires per-user CLI licenses available from [duplicacy.com](https://duplicacy.com/buy) at a cost of $20 per year
* A user is defined as the computer account that creates or edits the files to be backed up; if a backup contains files created or edited by multiple users for commercial purposes, one CLI license is required for each user
* The computer with a valid commercial license for the GUI version may run the CLI version without a CLI license
* CLI licenses are not required to restore or manage backups; only the backup command requires valid CLI licenses
* Modification and redistribution are permitted, but commercial use of derivative works is subject to the same requirements of this license

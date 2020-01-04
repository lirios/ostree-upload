/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

use crate::api;
use crate::errors::GenericError;
use log::info;
use ostree;
use std::collections::HashMap;
use std::fs;
use std::path::Path;
use std::path::PathBuf;

pub struct Receiver {
    repo_path: PathBuf,
    temp_path: PathBuf,
}

impl Receiver {
    pub fn new(repo_path: &Path) -> Result<Receiver, GenericError> {
        // Create temporary directory
        let temp_path = repo_path.join(".tmp");
        fs::create_dir_all(&temp_path).map_err(|e| {
            GenericError::new(&format!("Failed to create temporary directory: {}", e))
        })?;

        Ok(Receiver {
            repo_path: repo_path.to_owned(),
            temp_path: temp_path.to_owned(),
        })
    }

    fn open_repo(&self) -> Result<ostree::Repo, GenericError> {
        // We can't save an ostree::Repo instance inside Receiver because
        // it cannot be assigned to threads, so we reopen it every time
        // we need it
        let cancellable = gio::Cancellable::new();
        let repo = ostree::Repo::new_for_path(&self.repo_path);
        repo.open(Some(&cancellable))
            .map_err(|e| GenericError::new(&format!("Failed to open the repository: {}", e)))?;
        Ok(repo)
    }

    pub fn temp_path(&self, filename: &str) -> PathBuf {
        self.temp_path.join(&filename)
    }

    pub fn obj_path(&self, filename: &str) -> PathBuf {
        self.repo_path
            .join("objects")
            .join(&filename[..2])
            .join(&filename[2..])
    }

    pub fn get_info(&self) -> Result<api::Info, GenericError> {
        let repo = self.open_repo()?;

        let mode = match repo.get_mode() {
            ostree::RepoMode::Bare => "bare",
            ostree::RepoMode::Archive => "archive",
            ostree::RepoMode::BareUser => "bare-user",
            ostree::RepoMode::BareUserOnly => "bare-user-only",
            _ => "unknown",
        };

        let cancellable = gio::Cancellable::new();
        let refs = repo
            .list_refs(None, Some(&cancellable))
            .map_err(|e| GenericError::new(&format!("Failed to list refs: {}", e)))?;

        Ok(api::Info {
            mode: String::from(mode),
            refs: refs,
        })
    }

    pub fn check_update(
        &self,
        refs: HashMap<String, (String, String)>,
    ) -> Result<api::Status, GenericError> {
        let repo = self.open_repo()?;

        for (branch, revs) in refs {
            // See if branch can be updated (pass allow_noent=false otherwise it will
            // crash when the branch doesn't exist)
            match repo.resolve_rev(&branch, false) {
                Err(_) => {
                    // Branch cannot be resolved on the client end
                    if revs.0 != api::REV_NULL {
                        return Ok(api::Status {
                            status: false,
                            message: Some(format!(
                                "Invalid from commit {} for new branch {}",
                                &revs.0, &branch
                            )),
                        });
                    }
                }
                Ok(current) => {
                    if revs.0 != current {
                        return Ok(api::Status {
                            status: false,
                            message: Some(format!(
                                "Branch {} is at {}, not {}",
                                &branch, &current, &revs.0
                            )),
                        });
                    }
                }
            }
        }

        Ok(api::Status {
            status: true,
            message: None,
        })
    }

    pub fn update_refs(&self, refs: HashMap<String, (String, String)>) -> Result<(), GenericError> {
        let repo = self.open_repo()?;
        let cancellable = gio::Cancellable::new();

        for (branch, revs) in refs {
            info!(
                "Setting branch {} revision from {} to {}",
                &branch, &revs.0, &revs.1
            );
            repo.set_ref_immediate(None, &branch, Some(&revs.1), Some(&cancellable))
                .map_err(|e| {
                    GenericError::new(&format!(
                        "Failed to set branch {} revision from {} to {}: {}",
                        &branch, &revs.0, &revs.1, e
                    ))
                })?;
        }

        Ok(())
    }
}

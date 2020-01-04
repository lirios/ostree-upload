/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

use crate::api;
use crate::errors::GenericError;
use gio;
use glib;
use log::info;
use ostree;
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::fs::File;
use std::io;
use std::path::Path;
use std::path::PathBuf;
use std::result::Result;

pub struct Pusher {
    repo: ostree::Repo,
    repo_path: PathBuf,
    branches: HashMap<String, String>,
}

impl Pusher {
    pub fn new(repo_path: &Path, refs: Option<Vec<&str>>) -> Result<Pusher, glib::Error> {
        // Open repository
        let cancellable = gio::Cancellable::new();
        let repo = ostree::Repo::new_for_path(&repo_path);
        repo.open(Some(&cancellable))?;

        // Enumerate branches to push
        let mut branches = HashMap::new();
        if refs.is_some() {
            for refspec in refs.unwrap() {
                let rev = repo.resolve_rev(&refspec, false)?;
                branches.insert(refspec.to_string(), rev.to_string());
            }
        } else {
            for (branch, rev) in repo.list_refs(None, Some(&cancellable))? {
                branches.insert(branch, rev);
            }
        }

        Ok(Pusher {
            repo: repo,
            repo_path: repo_path.to_owned(),
            branches: branches,
        })
    }

    fn ostree_object_path(&self, object_name: &str) -> PathBuf {
        self.repo_path
            .join("objects")
            .join(&object_name[..2])
            .join(&object_name[2..])
    }

    fn needed_commits(
        &self,
        remote_rev: &str,
        local_rev: &str,
        commits: &mut Vec<String>,
    ) -> Result<(), GenericError> {
        let mut parent = Some(local_rev.to_string());
        let remote = if remote_rev == api::REV_NULL {
            None
        } else {
            Some(remote_rev.to_string())
        };

        while parent != remote {
            let parent_str = parent.unwrap().to_string();

            commits.push(parent_str.to_owned());

            match self
                .repo
                .load_variant_if_exists(ostree::ObjectType::Commit, &parent_str)
            {
                Err(_) => {
                    return Err(GenericError::new(&format!(
                        "Shallow history from commit {} doesn't contain remote commit {}",
                        &local_rev, &remote_rev
                    )));
                }
                Ok(commit) => {
                    parent = ostree::commit_get_parent(&commit).map(|s| s.as_str().to_string());
                    if parent.is_none() {
                        break;
                    }
                }
            }
        }

        if remote.is_some() && parent != remote {
            return Err(GenericError::new(&format!(
                "Remote commit {} not descendent of commit {}",
                &remote_rev, &local_rev
            )));
        }

        Ok(())
    }

    fn calculate_checksum(&self, path: &Path) -> io::Result<String> {
        let mut file = File::open(&path)?;
        let mut hasher = Sha256::new();
        io::copy(&mut file, &mut hasher)?;
        let hash = hasher.result();
        let hex = hash.as_ref().iter().map(|b| format!("{:x}", b)).collect();
        Ok(hex)
    }

    fn needed_objects(&self, revs: &Vec<String>) -> Result<Vec<api::NeededObject>, GenericError> {
        let mut objects: Vec<api::NeededObject> = Vec::new();
        let cancellable = gio::Cancellable::new();

        for rev in revs {
            match self.repo.traverse_commit(&rev, 0, Some(&cancellable)) {
                Err(_) => {
                    break;
                }
                Ok(reachable) => {
                    for object in reachable {
                        if let Some(object_name) =
                            ostree::object_to_string(object.checksum(), object.object_type())
                        {
                            match object.object_type() {
                                ostree::ObjectType::File => {
                                    // Make this a filez since we're archive-z2
                                    let file_object_name = format!("{}z", &object_name).to_string();
                                    let file_object_path =
                                        self.ostree_object_path(&file_object_name);
                                    let checksum = self
                                        .calculate_checksum(&file_object_path)
                                        .map_err(|e| GenericError::new(&format!("{}", e)))?;
                                    objects.push(api::NeededObject {
                                        rev: object.checksum().to_string(),
                                        object_name: file_object_name.to_owned(),
                                        object_path: file_object_path,
                                        checksum: checksum,
                                    });
                                }
                                ostree::ObjectType::Commit => {
                                    // Add in detached metadata
                                    let meta_object_name = &format!("{}meta", &object_name);
                                    let meta_object_path =
                                        self.ostree_object_path(&meta_object_name);
                                    let checksum = self
                                        .calculate_checksum(&meta_object_path)
                                        .map_err(|e| GenericError::new(&format!("{}", e)))?;
                                    if meta_object_path.exists() {
                                        objects.push(api::NeededObject {
                                            rev: object.checksum().to_string(),
                                            object_name: meta_object_name.to_owned(),
                                            object_path: meta_object_path,
                                            checksum: checksum,
                                        });
                                    }
                                }
                                _ => {
                                    let object_path = self.ostree_object_path(&object_name);
                                    let checksum = self
                                        .calculate_checksum(&object_path)
                                        .map_err(|e| GenericError::new(&format!("{}", e)))?;
                                    objects.push(api::NeededObject {
                                        rev: object.checksum().to_string(),
                                        object_name: object_name.to_owned(),
                                        object_path: object_path,
                                        checksum: checksum,
                                    });
                                }
                            }
                        }
                    }
                }
            }
        }

        Ok(objects)
    }

    pub fn check_update(
        &self,
        remote_refs: HashMap<String, String>,
    ) -> HashMap<String, (String, String)> {
        let mut update_refs: HashMap<String, (String, String)> = HashMap::new();
        let null_rev = api::REV_NULL.to_string();
        for (branch, rev) in &self.branches {
            let remote_rev = remote_refs.get(branch).unwrap_or(&null_rev);
            if rev != remote_rev {
                update_refs.insert(
                    branch.to_string(),
                    (remote_rev.to_string(), rev.to_string()),
                );
            }
        }

        update_refs
    }

    pub fn retrieve(
        &self,
        update_refs: &HashMap<String, (String, String)>,
    ) -> Result<Vec<api::NeededObject>, GenericError> {
        let mut commits: Vec<String> = Vec::new();
        for (branch, revs) in update_refs {
            info!(
                "Updating branch {} from {} to {}",
                &branch, &revs.0, &revs.1
            );
            self.needed_commits(&revs.0, &revs.1, &mut commits)?;
        }

        info!("Enumerating objects to send...");
        let objects = self.needed_objects(&commits)?;

        Ok(objects)
    }

    pub fn prune(&self) -> Result<(), GenericError> {
        let cancellable = gio::Cancellable::new();
        let (found, pruned, size) = self
            .repo
            .prune(ostree::RepoPruneFlags::None, -1, Some(&cancellable))
            .map_err(|e| GenericError::new(&format!("{}", e)))?;
        info!(
            "Pruned {}/{} objects, {} bytes deleted",
            pruned, found, size
        );

        Ok(())
    }
}

/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

extern crate ostreereceiver;

use clap::{App, Arg};
use env_logger;
use indicatif::{ProgressBar, ProgressStyle};
use log::error;
use log::info;
use ostreereceiver::api;
use ostreereceiver::client::OstreeUploadClient;
use ostreereceiver::pusher::Pusher;
use std::collections::HashMap;
use std::env;
use std::path::Path;
use std::process::exit;

fn main() {
    if env::var("RUST_LOG").is_err() {
        env::set_var("RUST_LOG", "info");
    }
    env_logger::builder()
        .format_timestamp(None)
        .format_level(false)
        .format_module_path(false)
        .init();

    // Command line arguments
    let matches = App::new("oic")
        .version(env!("CARGO_PKG_VERSION"))
        .about("Updates a production OSTree repository.")
        .author("Pier Luigi Fiorini")
        .arg(
            Arg::with_name("repodir")
                .long("repo")
                .value_name("DIRECTORY")
                .required(true)
                .takes_value(true)
                .help("OSTree repository"),
        )
        .arg(
            Arg::with_name("url")
                .long("url")
                .value_name("URL")
                .default_value("http://127.0.0.1:8080")
                .required(true)
                .takes_value(true)
                .help("Server URL"),
        )
        .arg(
            Arg::with_name("refs")
                .long("ref")
                .value_name("REFSPEC")
                .takes_value(true)
                .multiple(true)
                .help("Ref to upload to the production server"),
        )
        .get_matches();

    let repodir = matches.value_of("repodir").unwrap();
    let url = matches.value_of("url").unwrap();

    let refs: Option<Vec<&str>> = if matches.is_present("refs") {
        Some(matches.values_of("refs").unwrap().collect())
    } else {
        None
    };

    let pusher = match Pusher::new(Path::new(&repodir), refs) {
        Err(e) => {
            eprintln!("{}", e);
            exit(1);
        }
        Ok(pusher) => pusher,
    };

    let client = OstreeUploadClient::new(&url);

    // Repository information
    info!("Receiving repository information...");
    let info: api::Info = match client.get_info() {
        Err(e) => {
            error!("Failed to get repository information: {}", e);
            exit(1);
        }
        Ok(info) => info,
    };
    if info.mode != "archive" {
        error!("Can only push to repositories in 'archive' mode");
        exit(1);
    }

    // See if there is something to update
    info!("Looking for branches to update...");
    let update_refs: HashMap<String, (String, String)> = pusher.check_update(info.refs);
    if update_refs.len() == 0 {
        println!("Nothing to update");
        exit(0);
    }
    let response = match client.update(&update_refs) {
        Err(e) => {
            error!("Failed to update refs: {}", e);
            exit(1);
        }
        Ok(response) => response,
    };
    if !response.status {
        if response.message.is_some() {
            error!("Update failed: {}", response.message.unwrap());
        } else {
            error!("Failed to update");
        }
        exit(1);
    }

    // Prune the repository before sending any object
    match pusher.prune() {
        Err(e) => {
            error!("Failed to prune repository: {}", e);
            exit(1);
        }
        _ => {}
    }

    // Collect commits and objects to push
    let needed_objects = match pusher.retrieve(&update_refs) {
        Err(e) => {
            error!("Failed to collect commits and objects to push: {}", e);
            exit(1);
        }
        Ok(needed_objects) => needed_objects,
    };

    // Needed objects are a lot (even thousands) and the request can easily
    // be very large, to avoid errors such as 413 Payload Too Large, we
    // divide it in chunks
    let mut missing_objects = vec![];
    for chunk in needed_objects
        .chunks(100)
        .map(|c| c.iter().cloned().collect::<Vec<api::NeededObject>>())
    {
        // Check which objects have not been previously transferred
        let mut mo = match client.missing_objects(&chunk) {
            Err(e) => {
                error!("Failed to check which objects need to be pushed: {}", e);
                exit(1);
            }
            Ok(response) => response.missing,
        };
        missing_objects.append(&mut mo);
    }

    // Send objecs
    info!("About to send {} objects...", missing_objects.len());
    let pb = ProgressBar::new(missing_objects.len() as u64);
    pb.set_style(
        ProgressStyle::default_bar()
            .template("{spinner:.green} [{elapsed_precise}] [{bar:40.cyan/blue}] {pos}/{len}")
            .progress_chars("#>-"),
    );
    let mut object_index: u64 = 0;
    for object in missing_objects {
        match client.upload(&object) {
            Err(e) => {
                error!("Failed to upload object {:?}: {}", &object.object_path, e);
                exit(1);
            }
            _ => {
                object_index += 1;
                pb.set_position(object_index);
            }
        }
    }

    // Update refs
    info!("Updating refs...");
    match client.done() {
        Err(e) => {
            error!("Failed to update refs: {}", e);
            exit(1);
        }
        _ => {}
    }
}

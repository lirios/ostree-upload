/****************************************************************************
 * Copyright (C) 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 ***************************************************************************/

use crate::api;
use crate::errors::GenericError;

use reqwest;
use std::borrow::Cow;
use std::collections::HashMap;

pub struct OstreeUploadClient {
    url: String,
}

impl OstreeUploadClient {
    pub fn new(url: &str) -> OstreeUploadClient {
        OstreeUploadClient {
            url: url.to_string(),
        }
    }

    pub fn get_info(&self) -> Result<api::Info, GenericError> {
        let response: api::Info = reqwest::blocking::Client::new()
            .get(&format!("{}/api/v1/info", &self.url))
            .bearer_auth("token")
            .header("User-Agent", "ostree-upload")
            .send()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .error_for_status()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .json()
            .map_err(|e| GenericError::new(&format!("{}", e)))?;
        Ok(response)
    }

    pub fn update(
        &self,
        refs: &HashMap<String, (String, String)>,
    ) -> Result<api::Status, GenericError> {
        let request = api::UpdateRequest { refs: refs.clone() };
        let response: api::Status = reqwest::blocking::Client::new()
            .post(&format!("{}/api/v1/update", &self.url))
            .bearer_auth("token")
            .header("User-Agent", "ostree-upload")
            .json(&request)
            .send()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .error_for_status()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .json()
            .map_err(|e| GenericError::new(&format!("{}", e)))?;
        Ok(response)
    }

    pub fn missing_objects(
        &self,
        objects: &Vec<api::NeededObject>,
    ) -> Result<api::MissingObjectsResponse, GenericError> {
        let request = api::MissingObjectsArgs {
            wanted: objects.to_vec(),
        };
        let response: api::MissingObjectsResponse = reqwest::blocking::Client::new()
            .get(&format!("{}/api/v1/missing_objects", &self.url))
            .bearer_auth("token")
            .header("User-Agent", "ostree-upload")
            .json(&request)
            .send()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .error_for_status()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .json()
            .map_err(|e| GenericError::new(&format!("{}", e)))?;
        Ok(response)
    }

    pub fn upload(&self, object: &api::NeededObject) -> Result<api::Status, GenericError> {
        let form = reqwest::blocking::multipart::Form::new()
            .text("rev", Cow::Owned(object.rev.to_owned()))
            .text("object_name", Cow::Owned(object.object_name.to_owned()))
            .text("checksum", Cow::Owned(object.checksum.to_owned()))
            .file("file", &object.object_path)
            .map_err(|e| GenericError::new(&format!("{}", e)))?;
        let response: api::Status = reqwest::blocking::Client::new()
            .post(&format!("{}/api/v1/upload", &self.url))
            .bearer_auth("token")
            .header("User-Agent", "ostree-upload")
            .multipart(form)
            .send()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .error_for_status()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .json()
            .map_err(|e| GenericError::new(&format!("{}", e)))?;
        Ok(response)
    }

    pub fn done(&self) -> Result<api::Status, GenericError> {
        let response: api::Status = reqwest::blocking::Client::new()
            .post(&format!("{}/api/v1/done", &self.url))
            .bearer_auth("token")
            .header("User-Agent", "ostree-upload")
            .send()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .error_for_status()
            .map_err(|e| GenericError::new(&format!("{}", e)))?
            .json()
            .map_err(|e| GenericError::new(&format!("{}", e)))?;
        Ok(response)
    }
}

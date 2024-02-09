/*
Copyright [2023] [Amrudesh Balakrishnan]

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/package apk

import (
	"encoding/json"
	"io/ioutil"
	database "morf/db"
	"morf/models"
	util "morf/utils"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	vip "github.com/spf13/viper"
	"gorm.io/gorm"
)

func StartCliExtraction(apkPath string, db *gorm.DB, is_db_req bool) {
	if is_db_req {
		apkFound, json_data := util.CheckDuplicateInDB(db, apkPath)
		if apkFound {
			log.Info("APK already exists in the database")
			log.Info(json_data)
		}
	}
	packageModel := ExtractPackageData(apkPath)
	metadata := StartMetaDataCollection(apkPath)
	scanner_data := StartSecScan("temp/input/" + apkPath)
	secret_data, secret_error := json.Marshal(scanner_data)

	if secret_error != nil {
		log.Error(secret_error)
	}

	secret := util.CreateSecretModel(apkPath, packageModel, metadata, scanner_data, secret_data)
	if is_db_req {
		database.InsertSecrets(secret, db)
	}
	json_data, json_error := json.MarshalIndent(secret, "", " ")

	if json_error != nil {
		log.Error(json_error)
	}

	_, err_ := os.Stat(vip.GetString("backup_path"))

	if os.IsNotExist(err_) {
		os.Mkdir(vip.GetString("backup_path"), 0755)
	}

	err := ioutil.WriteFile(vip.GetString("backup_path")+"/"+apkPath+"_"+secret.APKVersion+".json", json_data, 0644)
	if err != nil {
		log.Error(err)
	}

	log.Info("APK Data saved to: " + vip.GetString("backup_path") + "/" + apkPath + "_" + secret.APKVersion + ".json")
}

func StartJiraProcess(jiramodel models.JiraModel, db *gorm.DB, c *gin.Context) {
	apk_path := util.DownloadFileUsingSlack(jiramodel, c)
	if apk_path == "" {
		return
	}

	apkFound, json_data := util.CheckDuplicateInDB(db, apk_path)

	if apkFound {
		log.Info("APK already exists in the database")
		var secrets models.Secrets
		apk_data := json.Unmarshal([]byte(json_data), &secrets)
		if apk_data != nil {
			log.Error(apk_data)
		}
		util.CookJiraComment(jiramodel, secrets, c)
		return
	}

	packageModel := ExtractPackageData(apk_path)
	metadata := StartMetaDataCollection(apk_path)
	scanner_data := StartSecScan("temp/input/" + apk_path)
	secret_data, secret_error := json.Marshal(scanner_data)

	if secret_error != nil {
		log.Error(secret_error)
	}
	secret := util.CreateSecretModel(apk_path, packageModel, metadata, scanner_data, secret_data)
	database.InsertSecrets(secret, db)

	// Comment the data to JIRA ticket
	util.CookJiraComment(jiramodel, secret, c)
}

func StartExtractProcess(apkPath string, db *gorm.DB, c *gin.Context, isSlack bool, slackData models.SlackData) {

	apkFound, json_data := util.CheckDuplicateInDB(db, apkPath)
	if apkFound {
		if isSlack {
			util.RespondSecretsToSlack(slackData, c, string(json_data))
		} else {

			c.JSON(http.StatusOK, gin.H{
				"status":  http.StatusOK,
				"message": "APK already in database",
				"data":    string(json_data),
			})
		}
		return
	}

	packageModel := ExtractPackageData(apkPath)
	metadata := StartMetaDataCollection(apkPath)
	scanner_data := StartSecScan("temp/input/" + apkPath)
	secret_data, secret_error := json.Marshal(scanner_data)

	if secret_error != nil {
		log.Error(secret_error)
	}

	secret := util.CreateSecretModel(apkPath, packageModel, metadata, scanner_data, secret_data)

	database.InsertSecrets(secret, db)

	json_data, json_error := json.MarshalIndent(secret, "", " ")

	if json_error != nil {
		log.Error("JSON ERROR: ", json_error)
		log.Error(json_error)
	}

	//Check if backup folder exists
	_, err_ := os.Stat(vip.GetString("backup_path"))

	if os.IsNotExist(err_) {
		os.Mkdir(vip.GetString("backup_path"), 0755)
	}

	// Check if file exists

	//Move the APK Data to backup folder
	backupPath := vip.GetString("backup_path") + apkPath + "_" + secret.APKVersion + ".json"
	log.Println("Backup Path: ", backupPath)
	err := ioutil.WriteFile(backupPath, json_data, 0644)

	if err != nil {
		log.Error(err)
	}

	if !isSlack {
		c.JSON(http.StatusOK, gin.H{
			"message": "Success",
			"data":    string(json_data),
		})
	}

	if isSlack {
		util.RespondSecretsToSlack(slackData, c, string(json_data))
	}

}

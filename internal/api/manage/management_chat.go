/*
** description("").
** copyright('open-im,www.open-im.io').
** author("fg,Gordon@tuoyun.net").
** time(2021/9/15 15:23).
 */
package manage

import (
	api "Open_IM/pkg/base_info"
	"Open_IM/pkg/common/config"
	"Open_IM/pkg/common/constant"
	"Open_IM/pkg/common/log"
	"Open_IM/pkg/common/token_verify"
	"Open_IM/pkg/grpc-etcdv3/getcdv3"
	pbChat "Open_IM/pkg/proto/chat"
	"Open_IM/pkg/proto/sdk_ws"
	open_im_sdk "Open_IM/pkg/proto/sdk_ws"
	"Open_IM/pkg/utils"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/golang/protobuf/proto"
	"github.com/mitchellh/mapstructure"
	"net/http"
	"strings"
)

var validate *validator.Validate

func newUserSendMsgReq(params *api.ManagementSendMsgReq) *pbChat.SendMsgReq {
	var newContent string
	var err error
	switch params.ContentType {
	case constant.Text:
		newContent = params.Content["text"].(string)
	case constant.Picture:
		fallthrough
	case constant.Custom:
		fallthrough
	case constant.Voice:
		fallthrough
	case constant.Video:
		fallthrough
	case constant.File:
		newContent = utils.StructToJsonString(params.Content)
	case constant.Revoke:
		newContent = params.Content["revokeMsgClientID"].(string)
	default:
	}
	options := make(map[string]bool, 5)
	if params.IsOnlineOnly {
		utils.SetSwitchFromOptions(options, constant.IsOfflinePush, false)
		utils.SetSwitchFromOptions(options, constant.IsHistory, false)
		utils.SetSwitchFromOptions(options, constant.IsPersistent, false)
		utils.SetSwitchFromOptions(options, constant.IsSenderSync, false)
	}
	pbData := pbChat.SendMsgReq{
		OperationID: params.OperationID,
		MsgData: &open_im_sdk.MsgData{
			SendID:           params.SendID,
			GroupID:          params.GroupID,
			ClientMsgID:      utils.GetMsgID(params.SendID),
			SenderPlatformID: params.SenderPlatformID,
			SenderNickname:   params.SenderNickname,
			SenderFaceURL:    params.SenderFaceURL,
			SessionType:      params.SessionType,
			MsgFrom:          constant.SysMsgType,
			ContentType:      params.ContentType,
			Content:          []byte(newContent),
			RecvID:           params.RecvID,
			//	ForceList:        params.ForceList,
			CreateTime:      utils.GetCurrentTimestampByMill(),
			Options:         options,
			OfflinePushInfo: params.OfflinePushInfo,
		},
	}
	if params.ContentType == constant.OANotification {
		var tips open_im_sdk.TipsComm
		tips.JsonDetail = utils.StructToJsonString(params.Content)
		pbData.MsgData.Content, err = proto.Marshal(&tips)
		if err != nil {
			log.Error(params.OperationID, "Marshal failed ", err.Error(), tips.String())
		}
	}
	return &pbData
}
func init() {
	validate = validator.New()
}

// @Summary 管理员发送/撤回消息
// @Description 管理员发送/撤回消息 消息格式详细见<a href="https://doc.rentsoft.cn/#/server_doc/admin?id=%e6%b6%88%e6%81%af%e7%b1%bb%e5%9e%8b%e6%a0%bc%e5%bc%8f%e6%8f%8f%e8%bf%b0">消息格式</href>
// @Tags 消息相关
// @ID ManagementSendMsg
// @Accept json
// @Param token header string true "im token"
// @Param 管理员发送文字消息 body api.ManagementSendMsgReq{content=TextElem{}} true "该请求和消息结构体一样"
// @Param 管理员发送OA通知消息 body api.ManagementSendMsgReq{content=OANotificationElem{}} true "该请求和消息结构体一样"
// @Param 管理员撤回单聊消息 body api.ManagementSendMsgReq{content=RevokeElem{}} true "该请求和消息结构体一样"
// @Produce json
// @Success 0 {object} api.ManagementSendMsgResp "serverMsgID为服务器消息ID <br> clientMsgID为客户端消息ID <br> sendTime为发送消息时间"
// @Failure 500 {object} api.ManagementSendMsgResp "errCode为500 一般为服务器内部错误"
// @Failure 400 {object} api.ManagementSendMsgResp "errCode为400 一般为参数输入错误, token未带上等"
// @Router /msg/manage_send_msg [post]
func ManagementSendMsg(c *gin.Context) {
	var data interface{}
	params := api.ManagementSendMsgReq{}
	if err := c.BindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		log.Error(c.PostForm("operationID"), "json unmarshal err", err.Error(), c.PostForm("content"))
		return
	}
	switch params.ContentType {
	case constant.Text:
		data = TextElem{}
	case constant.Picture:
		data = PictureElem{}
	case constant.Voice:
		data = SoundElem{}
	case constant.Video:
		data = VideoElem{}
	case constant.File:
		data = FileElem{}
	//case constant.AtText:
	//	data = AtElem{}
	//case constant.Merger:
	//	data =
	//case constant.Card:
	//case constant.Location:
	case constant.Custom:
		data = CustomElem{}
	case constant.Revoke:
		data = RevokeElem{}
	case constant.OANotification:
		data = OANotificationElem{}
		params.SessionType = constant.NotificationChatType
	//case constant.HasReadReceipt:
	//case constant.Typing:
	//case constant.Quote:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 404, "errMsg": "contentType err"})
		log.Error(c.PostForm("operationID"), "contentType err", c.PostForm("content"))
		return
	}
	if err := mapstructure.WeakDecode(params.Content, &data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 401, "errMsg": err.Error()})
		log.Error(c.PostForm("operationID"), "content to Data struct  err", err.Error())
		return
	} else if err := validate.Struct(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 403, "errMsg": err.Error()})
		log.Error(c.PostForm("operationID"), "data args validate  err", err.Error())
		return
	}
	log.NewInfo(params.OperationID, data, params)
	token := c.Request.Header.Get("token")
	claims, err := token_verify.ParseToken(token, params.OperationID)
	if err != nil {
		log.NewError(params.OperationID, "parse token failed", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "parse token failed", "sendTime": 0, "MsgID": ""})
		return
	}
	if !utils.IsContain(claims.UID, config.Config.Manager.AppManagerUid) {
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "not authorized", "sendTime": 0, "MsgID": ""})
		return

	}
	switch params.SessionType {
	case constant.SingleChatType:
		if len(params.RecvID) == 0 {
			log.NewError(params.OperationID, "recvID is a null string")
			c.JSON(http.StatusBadRequest, gin.H{"errCode": 405, "errMsg": "recvID is a null string", "sendTime": 0, "MsgID": ""})
			return
		}
	case constant.GroupChatType:
		if len(params.GroupID) == 0 {
			log.NewError(params.OperationID, "groupID is a null string")
			c.JSON(http.StatusBadRequest, gin.H{"errCode": 405, "errMsg": "groupID is a null string", "sendTime": 0, "MsgID": ""})
			return
		}

	}
	log.NewInfo(params.OperationID, "Ws call success to ManagementSendMsgReq", params)

	pbData := newUserSendMsgReq(&params)
	log.Info(params.OperationID, "", "api ManagementSendMsg call start..., [data: %s]", pbData.String())

	etcdConn := getcdv3.GetConn(config.Config.Etcd.EtcdSchema, strings.Join(config.Config.Etcd.EtcdAddr, ","), config.Config.RpcRegisterName.OpenImOfflineMessageName, params.OperationID)
	if etcdConn == nil {
		errMsg := params.OperationID + "getcdv3.GetConn == nil"
		log.NewError(params.OperationID, errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{"errCode": 500, "errMsg": errMsg})
		return
	}
	client := pbChat.NewChatClient(etcdConn)

	log.Info(params.OperationID, "", "api ManagementSendMsg call, api call rpc...")

	RpcResp, err := client.SendMsg(context.Background(), pbData)
	if err != nil {
		log.NewError(params.OperationID, "call delete UserSendMsg rpc server failed", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"errCode": 500, "errMsg": "call UserSendMsg  rpc server failed"})
		return
	}
	log.Info(params.OperationID, "", "api ManagementSendMsg call end..., [data: %s] [reply: %s]", pbData.String(), RpcResp.String())
	resp := api.ManagementSendMsgResp{CommResp: api.CommResp{ErrCode: RpcResp.ErrCode, ErrMsg: RpcResp.ErrMsg}, ResultList: server_api_params.UserSendMsgResp{ServerMsgID: RpcResp.ServerMsgID, ClientMsgID: RpcResp.ClientMsgID, SendTime: RpcResp.SendTime}}
	log.Info(params.OperationID, "ManagementSendMsg return", resp)
	c.JSON(http.StatusOK, resp)
}

// @Summary 管理员批量发送群聊单聊消息
// @Description 管理员批量发送群聊单聊消息 消息格式详细见<a href="https://doc.rentsoft.cn/#/server_doc/admin?id=%e6%b6%88%e6%81%af%e7%b1%bb%e5%9e%8b%e6%a0%bc%e5%bc%8f%e6%8f%8f%e8%bf%b0">消息格式</href>
// @Tags 消息相关
// @ID ManagementBatchSendMsg
// @Accept json
// @Param token header string true "im token"
// @Param 管理员批量发送单聊消息 body api.ManagementBatchSendMsgReq{content=TextElem{}} true "该请求和消息结构体一样 <br> recvIDList为接受消息的用户ID列表"
// @Param 管理员批量发送OA通知 body api.ManagementSendMsgReq{content=OANotificationElem{}} true "该请求和消息结构体一样 <br> recvIDList为接受消息的用户ID列表"
// @Produce json
// @Success 0 {object} api.ManagementBatchSendMsgReq "serverMsgID为服务器消息ID <br> clientMsgID为客户端消息ID <br> sendTime为发送消息时间"
// @Failure 500 {object} api.ManagementBatchSendMsgReq "errCode为500 一般为服务器内部错误"
// @Failure 400 {object} api.ManagementBatchSendMsgReq "errCode为400 一般为参数输入错误, token未带上等"
// @Router /msg/batch_send_msg [post]
func ManagementBatchSendMsg(c *gin.Context) {
	var data interface{}
	params := api.ManagementBatchSendMsgReq{}
	resp := api.ManagementBatchSendMsgResp{}
	resp.Data.FailedIDList = make([]string, 0)
	if err := c.BindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		log.Error(c.PostForm("operationID"), "json unmarshal err", err.Error(), c.PostForm("content"))
		return
	}
	switch params.ContentType {
	case constant.Text:
		data = TextElem{}
	case constant.Picture:
		data = PictureElem{}
	case constant.Voice:
		data = SoundElem{}
	case constant.Video:
		data = VideoElem{}
	case constant.File:
		data = FileElem{}
	//case constant.AtText:
	//	data = AtElem{}
	//case constant.Merger:
	//	data =
	//case constant.Card:
	//case constant.Location:
	case constant.Custom:
		data = CustomElem{}
	case constant.Revoke:
		data = RevokeElem{}
	case constant.OANotification:
		data = OANotificationElem{}
		params.SessionType = constant.NotificationChatType
	//case constant.HasReadReceipt:
	//case constant.Typing:
	//case constant.Quote:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 404, "errMsg": "contentType err"})
		log.Error(c.PostForm("operationID"), "contentType err", c.PostForm("content"))
		return
	}
	if err := mapstructure.WeakDecode(params.Content, &data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 401, "errMsg": err.Error()})
		log.Error(c.PostForm("operationID"), "content to Data struct  err", err.Error())
		return
	} else if err := validate.Struct(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 403, "errMsg": err.Error()})
		log.Error(c.PostForm("operationID"), "data args validate  err", err.Error())
		return
	}
	log.NewInfo(params.OperationID, data, params)
	token := c.Request.Header.Get("token")
	claims, err := token_verify.ParseToken(token, params.OperationID)
	if err != nil {
		log.NewError(params.OperationID, "parse token failed", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "parse token failed", "sendTime": 0, "MsgID": ""})
		return
	}
	if !utils.IsContain(claims.UID, config.Config.Manager.AppManagerUid) {
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "not authorized", "sendTime": 0, "MsgID": ""})
		return

	}
	log.NewInfo(params.OperationID, "Ws call success to ManagementSendMsgReq", params)
	for _, recvID := range params.RecvIDList {
		req := &api.ManagementSendMsgReq{
			ManagementSendMsg: params.ManagementSendMsg,
			RecvID:            recvID,
		}
		pbData := newUserSendMsgReq(req)
		pbData.MsgData.RecvID = recvID
		log.Info(params.OperationID, "", "api ManagementSendMsg call start..., ", pbData.String())
		etcdConn := getcdv3.GetConn(config.Config.Etcd.EtcdSchema, strings.Join(config.Config.Etcd.EtcdAddr, ","), config.Config.RpcRegisterName.OpenImOfflineMessageName, params.OperationID)
		if etcdConn == nil {
			errMsg := params.OperationID + "getcdv3.GetConn == nil"
			log.NewError(params.OperationID, errMsg)
			resp.Data.FailedIDList = append(resp.Data.FailedIDList, recvID)
			continue
		}
		client := pbChat.NewChatClient(etcdConn)
		rpcResp, err := client.SendMsg(context.Background(), pbData)
		if err != nil {
			log.NewError(params.OperationID, "call delete UserSendMsg rpc server failed", err.Error())
			resp.Data.FailedIDList = append(resp.Data.FailedIDList, recvID)
			continue
		}
		if rpcResp.ErrCode != 0 {
			log.NewError(params.OperationID, utils.GetSelfFuncName(), "rpc failed", pbData, rpcResp)
			resp.Data.FailedIDList = append(resp.Data.FailedIDList, recvID)
			continue
		}
		resp.Data.ResultList = append(resp.Data.ResultList, server_api_params.UserSendMsgResp{
			ServerMsgID: rpcResp.ServerMsgID,
			ClientMsgID: rpcResp.ClientMsgID,
			SendTime:    rpcResp.SendTime,
		})
	}

	log.NewInfo(params.OperationID, utils.GetSelfFuncName(), "resp: ", resp)
	c.JSON(http.StatusOK, resp)
}

type PictureBaseInfo struct {
	UUID   string `mapstructure:"uuid"`
	Type   string `mapstructure:"type" `
	Size   int64  `mapstructure:"size" `
	Width  int32  `mapstructure:"width" `
	Height int32  `mapstructure:"height"`
	Url    string `mapstructure:"url" `
}

type PictureElem struct {
	SourcePath      string          `mapstructure:"sourcePath"`
	SourcePicture   PictureBaseInfo `mapstructure:"sourcePicture"`
	BigPicture      PictureBaseInfo `mapstructure:"bigPicture" `
	SnapshotPicture PictureBaseInfo `mapstructure:"snapshotPicture"`
}
type SoundElem struct {
	UUID      string `mapstructure:"uuid"`
	SoundPath string `mapstructure:"soundPath"`
	SourceURL string `mapstructure:"sourceUrl"`
	DataSize  int64  `mapstructure:"dataSize"`
	Duration  int64  `mapstructure:"duration"`
}
type VideoElem struct {
	VideoPath      string `mapstructure:"videoPath"`
	VideoUUID      string `mapstructure:"videoUUID"`
	VideoURL       string `mapstructure:"videoUrl"`
	VideoType      string `mapstructure:"videoType"`
	VideoSize      int64  `mapstructure:"videoSize"`
	Duration       int64  `mapstructure:"duration"`
	SnapshotPath   string `mapstructure:"snapshotPath"`
	SnapshotUUID   string `mapstructure:"snapshotUUID"`
	SnapshotSize   int64  `mapstructure:"snapshotSize"`
	SnapshotURL    string `mapstructure:"snapshotUrl"`
	SnapshotWidth  int32  `mapstructure:"snapshotWidth"`
	SnapshotHeight int32  `mapstructure:"snapshotHeight"`
}
type FileElem struct {
	FilePath  string `mapstructure:"filePath"`
	UUID      string `mapstructure:"uuid"`
	SourceURL string `mapstructure:"sourceUrl"`
	FileName  string `mapstructure:"fileName"`
	FileSize  int64  `mapstructure:"fileSize"`
}
type AtElem struct {
	Text       string   `mapstructure:"text"`
	AtUserList []string `mapstructure:"atUserList"`
	IsAtSelf   bool     `mapstructure:"isAtSelf"`
}
type LocationElem struct {
	Description string  `mapstructure:"description"`
	Longitude   float64 `mapstructure:"longitude"`
	Latitude    float64 `mapstructure:"latitude"`
}
type CustomElem struct {
	Data        string `mapstructure:"data" validate:"required"`
	Description string `mapstructure:"description"`
	Extension   string `mapstructure:"extension"`
}
type TextElem struct {
	Text string `mapstructure:"text" validate:"required"`
}

type RevokeElem struct {
	RevokeMsgClientID string `mapstructure:"revokeMsgClientID" validate:"required"`
}
type OANotificationElem struct {
	NotificationName    string      `mapstructure:"notificationName" json:"notificationName" validate:"required"`
	NotificationFaceURL string      `mapstructure:"notificationFaceURL" json:"notificationFaceURL" validate:"required"`
	NotificationType    int32       `mapstructure:"notificationType" json:"notificationType" validate:"required"`
	Text                string      `mapstructure:"text" json:"text" validate:"required"`
	Url                 string      `mapstructure:"url" json:"url"`
	MixType             int32       `mapstructure:"mixType" json:"mixType"`
	PictureElem         PictureElem `mapstructure:"pictureElem" json:"pictureElem"`
	SoundElem           SoundElem   `mapstructure:"soundElem" json:"soundElem"`
	VideoElem           VideoElem   `mapstructure:"videoElem" json:"videoElem"`
	FileElem            FileElem    `mapstructure:"fileElem" json:"fileElem"`
	Ex                  string      `mapstructure:"ex" json:"ex"`
}

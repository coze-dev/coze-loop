namespace go coze.loop.data

include "coze.loop.data.dataset.thrift"
include "./coze.loop.data.tag.thrift"

service DatasetService extends coze.loop.data.dataset.DatasetService{}
service TagService extends coze.loop.data.tag.TagService{}
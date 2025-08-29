package data

// 任务执行的结果
// 对于小的结果，直接存入API-Server，在不同节点之间同步
// 对于大的结果，在本地构建Server,并对外暴露资源的访问方式，并存入API-Server
// TODO:

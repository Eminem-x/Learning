package main

func main() {
    // 先通过 machineFails 获取 machine
    if err := rds.DBConn.Preload("Machine").Find(&machineFails, failIDs).Error; err != nil {
        logs.Error(e.ErrPreSendMsg, err)
        return
    }
    // 再通过 machines 获取 station
    var machineIDs []uint
    var machines []models.Machine
    for _, v := range machineFails {
        machineIDs = append(machineIDs, v.Machine.ID)
        machines = append(machines, v.Machine)
    }
    if err := rds.DBConn.Preload("Station").Find(&machines, machineIDs).Error; err != nil {
        logs.Error(e.ErrPreSendMsg, err)
        return
    }
    // 拼接失败详情
    failReasons = ""
    for i := range machineFails {
        // province-city-siteName  "失败原因": fail
        failReasons += machines[i].Station.Province + "-"
        failReasons += machines[i].Station.City + "-"
        failReasons += machines[i].Station.SiteName + " "
        failReasons += "失败原因: " + machineFails[i].FailInfo
        if i != len(machineFails)-1 {
            failReasons += "\n"
        }
    }
}

# 坐标转换支持与菜单溢出修复实现文档

## 问题背景

原始 Gio 库在处理带有变换（如滚动列表）的上下文菜单时存在以下问题：

1. **右侧列表区域的右键菜单无法弹出** - 补丁修改了 `Defer` 函数，删除了关键的 `Save/Load` 状态恢复机制
2. **菜单位置偏移严重** - 补丁中的 `getTransform` 函数错误地重复累积变换矩阵
3. **菜单可能溢出窗口边界** - 原始实现使用局部坐标进行边界检查，无法正确处理嵌套变换的情况

## 问题分析

### 1. Defer 函数的 Save/Load 机制被破坏

**原始代码逻辑：**
```go
state := ops.Save(&o.Internal)
m := Record(o)
state.Load()  // 关键：恢复变换状态
c.Add(o)
c = m.Stop()
```

**补丁后的错误代码：**
```go
_ = ops.Save(&o.Internal)  // 丢弃了返回值
m := Record(o)
// state.Load() 被删除！
c.Add(o)
c = m.Stop()
```

`Save/Load` 是 Gio 内部的状态管理机制，用于确保延迟操作在正确的变换状态下执行。删除后导致延迟操作执行时变换状态错误。

### 2. getTransform 函数错误累积变换

`areaNode.trans` 存储的是**累积的全局变换**（已包含所有父级变换），但补丁错误地遍历父节点并重复乘以变换：

```go
// 错误实现
func (q *pointerQueue) getTransform(areaIdx int) f32.Affine2D {
    transform := f32.AffineId()
    for areaIdx != -1 {
        a := &q.areas[areaIdx]
        transform = transform.Mul(a.trans)  // 错误：重复累积
        areaIdx = a.parent
    }
    return transform
}

// 正确实现
func (q *pointerQueue) getTransform(areaIdx int) f32.Affine2D {
    if areaIdx == -1 {
        return f32.AffineId()
    }
    return q.areas[areaIdx].trans  // 直接返回已累积的变换
}
```

### 3. 窗口边界检查使用错误坐标系

原始实现使用局部坐标 `r.position` 进行边界检查，无法正确处理嵌套在滚动列表等变换容器中的菜单。

## 解决方案

### 修改文件列表

| 文件 | 修改内容 |
|------|----------|
| `op/op.go` | 修复 `Defer` 函数、修复 `Reset` 函数、添加 `GlobalTransform` |
| `io/pointer/pointer.go` | 添加 `Transform` 字段和 `AbsolutePosition()` 方法 |
| `io/input/pointer.go` | 添加 `getTransform` 函数、设置事件的 `Transform` 字段 |
| `x/component/context-area.go` | 使用绝对坐标进行边界检查和菜单定位 |

### 详细实现

#### 1. op/op.go - Defer 函数修复

```go
func Defer(o *Ops, c CallOp) {
    if c.ops == nil {
        return
    }

    // 保存当前变换状态用于自定义追踪
    savedCurrentTransform := o.currentTransform
    savedTransformStack := make([]f32.Affine2D, len(o.transformStack))
    copy(savedTransformStack, o.transformStack)

    // 关键：保存并恢复内部 ops 状态
    state := ops.Save(&o.Internal)

    m := Record(o)

    // 关键：在宏执行时恢复状态
    state.Load()

    // 恢复自定义变换追踪
    o.currentTransform = savedCurrentTransform
    o.transformStack = make([]f32.Affine2D, len(savedTransformStack))
    copy(o.transformStack, savedTransformStack)

    c.Add(o)
    c = m.Stop()

    // 宏录制完成后恢复变换追踪
    o.currentTransform = savedCurrentTransform
    o.transformStack = make([]f32.Affine2D, len(savedTransformStack))
    copy(o.transformStack, savedTransformStack)

    // 写入 Defer 操作码
    data := ops.Write(&o.Internal, ops.TypeDeferLen)
    data[0] = byte(ops.TypeDefer)
    c.Add(o)
}
```

#### 2. io/pointer/pointer.go - 添加坐标转换支持

```go
type Event struct {
    // ... 其他字段 ...

    // Transform 是用于将全局坐标转换为局部坐标的变换矩阵
    // 可以通过应用逆变换将局部坐标转换回全局坐标
    Transform f32.Affine2D
}

// AbsolutePosition 返回窗口绝对坐标
func (e Event) AbsolutePosition() f32.Point {
    return e.Transform.Transform(e.Position)
}
```

#### 3. io/input/pointer.go - 正确设置变换

```go
// getTransform 返回指定区域的累积变换
func (q *pointerQueue) getTransform(areaIdx int) f32.Affine2D {
    if areaIdx == -1 {
        return f32.AffineId()
    }
    // areas[areaIdx].trans 已经是累积的全局变换
    return q.areas[areaIdx].trans
}

// 在事件分发时设置 Transform
func (q *pointerQueue) deliverEvent(...) {
    // ...
    transform := q.getTransform(h.pointer.areaPlusOne - 1)
    e.Transform = transform
    e.Position = transform.Invert().Transform(e.Position)
    // ...
}
```

#### 4. x/component/context-area.go - 使用绝对坐标进行边界检查

```go
type ContextArea struct {
    // ...
    position         f32.Point       // 局部坐标（用于点击检测）
    absolutePosition f32.Point       // 窗口绝对坐标（用于边界检查）
    transform        f32.Affine2D    // 事件时的变换矩阵
    // ...
}

func (r *ContextArea) Update(gtx C) {
    // ...
    if e.Buttons.Contain(r.Activation) && e.Kind == pointer.Press {
        r.active = true
        r.justActivated = true
        if !r.AbsolutePosition {
            r.position = e.Position
            r.absolutePosition = e.AbsolutePosition()
            r.transform = e.Transform
        }
    }
    // ...
}

func (r *ContextArea) Layout(gtx C, w layout.Widget) D {
    // ...
    if r.active {
        windowSize := gtx.WindowSize
        menuWidth := r.dims.Size.X
        menuHeight := r.dims.Size.Y

        // 使用窗口绝对坐标进行边界检查
        absPosX := int(math.Round(float64(r.absolutePosition.X)))
        absPosY := int(math.Round(float64(r.absolutePosition.Y)))

        // 调整位置防止溢出窗口
        if absPosX+menuWidth > windowSize.X {
            absPosX = absPosX - menuWidth
            if absPosX < 0 {
                absPosX = 0
            }
        }
        if absPosY+menuHeight > windowSize.Y {
            absPosY = absPosY - menuHeight
            if absPosY < 0 {
                absPosY = 0
            }
        }

        // 使用保存的变换矩阵转换回局部坐标
        localPos := r.transform.Invert().Transform(f32.Pt(float32(absPosX), float32(absPosY)))

        pos := image.Point{
            X: int(math.Round(float64(localPos.X))),
            Y: int(math.Round(float64(localPos.Y))),
        }
        // ... 应用偏移并渲染菜单
    }
    // ...
}
```

## 坐标转换流程图

```
┌─────────────────────────────────────────────────────────────────┐
│                     用户点击事件                                  │
│                  (全局窗口坐标)                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              pointerQueue.deliverEvent()                        │
│                                                                 │
│  1. transform = getTransform(areaIdx)  // 获取累积变换           │
│  2. e.Transform = transform            // 保存变换到事件          │
│  3. e.Position = transform.Invert().Transform(globalPos)        │
│     // 将全局坐标转换为局部坐标                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              ContextArea.Update()                               │
│                                                                 │
│  1. r.position = e.Position            // 保存局部坐标           │
│  2. r.absolutePosition = e.AbsolutePosition()                   │
│     // = e.Transform.Transform(e.Position) = 全局坐标            │
│  3. r.transform = e.Transform          // 保存变换矩阵           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              ContextArea.Layout()                               │
│                                                                 │
│  1. 边界检查：使用 r.absolutePosition 和 gtx.WindowSize          │
│  2. 计算调整后的窗口绝对坐标 (adjustedAbsPos)                     │
│  3. 转换回局部坐标：                                              │
│     localPos = r.transform.Invert().Transform(adjustedAbsPos)   │
│  4. 应用 op.Offset(localPos) 渲染菜单                            │
└─────────────────────────────────────────────────────────────────┘
```

## 关键概念

### 1. 变换矩阵累积

Gio 中每个 `areaNode.trans` 存储的是**从局部坐标到窗口坐标的累积变换**：

```
全局坐标 = 局部坐标 × 累积变换矩阵
局部坐标 = 全局坐标 × 累积变换矩阵的逆
```

### 2. Defer 的状态恢复

`op.Defer` 使用 `Save/Load` 机制确保延迟操作在调用时的变换状态下执行：

```go
state := ops.Save(&o.Internal)  // 保存当前状态
m := Record(o)
state.Load()                     // 在宏执行时恢复状态
c.Add(o)
c = m.Stop()
```

### 3. 坐标系统

- **全局坐标（窗口坐标）**：原点在窗口左上角，用于边界检查
- **局部坐标**：相对于当前变换原点，用于布局和渲染

## 测试验证

修复后验证以下场景：

1. ✅ 左侧区域右键菜单正常弹出
2. ✅ 右侧滚动列表中的右键菜单正常弹出
3. ✅ 菜单位置正确跟随点击锚点
4. ✅ 菜单不会溢出窗口边界
5. ✅ 窗口边缘点击时菜单自动调整位置

## 后续优化建议

1. 清理调试日志（当前保留用于问题排查）
2. 考虑添加 `PositionHint` 的自动检测功能
3. 可以添加菜单最大尺寸限制，防止菜单过大溢出窗口

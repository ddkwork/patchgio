# 坐标转换支持与菜单溢出修复实现文档

## 概述

本文档描述 Gio UI 库中坐标转换系统的实现，用于支持带有变换（如滚动列表）的上下文菜单正确定位。

**核心机制：** `op/op.go` 中的 `Defer` 函数已包含正确的 `Save/Load` 状态恢复机制，确保延迟操作在正确的变换状态下执行。

## 核心组件

### 1. op/op.go - Defer 函数

```go
func Defer(o *Ops, c CallOp) {
    if c.ops == nil {
        return
    }
    state := ops.Save(&o.Internal)  // 保存当前状态
    m := Record(o)
    state.Load()                     // 恢复状态（关键！）
    c.Add(o)
    c = m.Stop()
    // ...
}
```

`Save/Load` 机制确保延迟操作在调用时的变换状态下执行，而非定义时。

### 2. io/pointer/pointer.go - 事件坐标转换

```go
type Event struct {
    // ... 其他字段 ...
    Position  f32.Point     // 局部坐标
    Transform f32.Affine2D  // 局部→全局变换矩阵
}

// AbsolutePosition 返回窗口绝对坐标
func (e Event) AbsolutePosition() f32.Point {
    return e.Transform.Transform(e.Position)
}
```

### 3. io/input/pointer.go - 变换获取

```go
// getTransform 返回指定区域的累积变换
// areas[areaIdx].trans 已经是累积的全局变换
func (q *pointerQueue) getTransform(areaIdx int) f32.Affine2D {
    if areaIdx == -1 {
        return f32.AffineId()
    }
    return q.areas[areaIdx].trans
}
```

在事件分发时设置 `Transform`：
```go
e.Transform = q.getTransform(h.pointer.areaPlusOne - 1)
e.Position = q.invTransform(h.pointer.areaPlusOne-1, e.Position)
```

### 4. x/component/context-area.go - 上下文菜单定位

```go
type ContextArea struct {
    position         f32.Point       // 局部坐标
    absolutePosition f32.Point       // 窗口绝对坐标
    transform        f32.Affine2D    // 事件时的变换矩阵
    // ...
}

func (r *ContextArea) Update(gtx C) {
    // 保存事件坐标信息
    if e.Buttons.Contain(r.Activation) && e.Kind == pointer.Press {
        r.position = e.Position
        r.absolutePosition = e.AbsolutePosition()
        r.transform = e.Transform
    }
}

func (r *ContextArea) Layout(gtx C, w layout.Widget) D {
    if r.active {
        windowSize := gtx.WindowSize
        menuWidth := r.dims.Size.X
        menuHeight := r.dims.Size.Y

        // 使用窗口绝对坐标进行边界检查
        absPosX := int(math.Round(float64(r.absolutePosition.X)))
        absPosY := int(math.Round(float64(r.absolutePosition.Y)))

        // 防止溢出窗口
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

        // 转换回局部坐标用于渲染
        localPos := r.transform.Invert().Transform(f32.Pt(float32(absPosX), float32(absPosY)))
        // 应用 op.Offset(localPos) 渲染菜单
    }
}
```

## 坐标转换流程

```
用户点击事件（全局窗口坐标）
         │
         ▼
┌─────────────────────────────────────────┐
│     pointerQueue.deliverEvent()         │
│  1. transform = getTransform(areaIdx)   │
│  2. e.Transform = transform             │
│  3. e.Position = transform.Invert()     │
│     .Transform(globalPos)               │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│     ContextArea.Update()                │
│  1. r.position = e.Position（局部）     │
│  2. r.absolutePosition =                │
│     e.AbsolutePosition()（全局）        │
│  3. r.transform = e.Transform           │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│     ContextArea.Layout()                │
│  1. 用 absolutePosition 检查窗口边界    │
│  2. 调整位置防止溢出                     │
│  3. transform.Invert().Transform()      │
│     转换回局部坐标                       │
│  4. 渲染菜单                            │
└─────────────────────────────────────────┘
```

## 关键概念

### 变换矩阵累积

每个 `areaNode.trans` 存储从局部坐标到窗口坐标的累积变换：

```
全局坐标 = 局部坐标 × 累积变换矩阵
局部坐标 = 全局坐标 × 累积变换矩阵的逆
```

### 坐标系统

- **全局坐标（窗口坐标）**：原点在窗口左上角，用于边界检查
- **局部坐标**：相对于当前变换原点，用于布局和渲染

## 修改文件清单

| 文件 | 修改内容 |
|------|----------|
| `io/pointer/pointer.go` | 添加 `Transform` 字段和 `AbsolutePosition()` 方法 |
| `io/input/pointer.go` | 添加 `getTransform` 函数，设置事件的 `Transform` |
| `x/component/context-area.go` | 添加窗口边界检查，使用绝对坐标定位 |
| `example/component/pages/menu/menu.go` | 添加列表项点击效果 |

## 验证场景

1. ✅ 左侧区域右键菜单正常弹出
2. ✅ 右侧滚动列表中的右键菜单正常弹出
3. ✅ 菜单位置正确跟随点击锚点
4. ✅ 菜单不会溢出窗口边界
5. ✅ 窗口边缘点击时菜单自动调整位置

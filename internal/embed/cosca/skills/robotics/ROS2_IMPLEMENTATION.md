# ROS2 + Robotics — Enterprise

> **Version**: 1.0.0 | **Stack**: ROS2 Humble, C++17, Gazebo, Nav2

## ROS2 Node (C++)

```cpp
// motor_controller.cpp
#include <rclcpp/rclcpp.hpp>
#include <geometry_msgs/msg/twist.hpp>
#include <sensor_msgs/msg/laser_scan.hpp>

class MotorController : public rclcpp::Node {
public:
  MotorController() : Node("motor_controller") {
    cmd_sub_ = create_subscription<geometry_msgs::msg::Twist>(
      "/cmd_vel", 10, [this](const auto& msg) { cmd_callback(msg); });

    scan_sub_ = create_subscription<sensor_msgs::msg::LaserScan>(
      "/scan", 10, [this](const auto& msg) { scan_callback(msg); });
  }

private:
  void cmd_callback(const geometry_msgs::msg::Twist& msg) {
    set_motor_pwm(msg.linear.x, msg.angular.z);
  }

  void scan_callback(const sensor_msgs::msg::LaserScan& msg) {
    if (min_distance(msg) < SAFETY_LIMIT) emergency_stop();
  }

  rclcpp::Subscription<geometry_msgs::msg::Twist>::SharedPtr cmd_sub_;
  rclcpp::Subscription<sensor_msgs::msg::LaserScan>::SharedPtr scan_sub_;
};
```

## Motor Control — PID

```cpp
// pid.h
class PID {
  float kp, ki, kd, integral = 0, prev_error = 0;
public:
  PID(float p, float i, float d) : kp(p), ki(i), kd(d) {}
  float update(float setpoint, float measurement, float dt) {
    float error = setpoint - measurement;
    integral += error * dt;
    float derivative = (error - prev_error) / dt;
    prev_error = error;
    return kp * error + ki * integral + kd * derivative;
  }
  void reset() { integral = 0; prev_error = 0; }
};
```

## Computer Vision Embedded

```python
# TensorFlow Lite on Coral TPU / Raspberry Pi
import tflite_runtime.interpreter as tflite
import cv2

interpreter = tflite.Interpreter("model_edgetpu.tflite",
    experimental_delegates=[tflite.load_delegate("libedgetpu.so.1")])
interpreter.allocate_tensors()

def detect(frame):
    blob = cv2.dnn.blobFromImage(frame, 1/255.0, (320,320), swapRB=True)
    interpreter.set_tensor(input_idx, blob)
    interpreter.invoke()
    return interpreter.get_tensor(output_idx)  # boxes, scores, classes
```

## Safety — Emergency Stop

```cpp
void emergency_stop() {
  digitalWrite(MOTOR_ENABLE, LOW);  // kill power to motors
  digitalWrite(ESTOP_LED, HIGH);    // indicate fault
  rclcpp::shutdown();               // stop ROS2
}

// Hardware watchdog — external IC (TPS3850) pulses WDI pin
// Software watchdog — FreeRTOS task watchdog with 100ms timeout
```

```bash
colcon build --symlink-install    # build ROS2 workspace
ros2 launch robot bringup.launch  # launch robot
ros2 topic echo /odom             # monitor odometry
```

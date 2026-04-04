// 计算器状态
let currentInput = '0';
let previousInput = '';
let operation = null;
let resetScreen = false;

// 获取显示元素
const display = document.getElementById('display');

// 更新显示屏
function updateDisplay() {
    display.textContent = currentInput;
}

// 输入数字
function inputNumber(number) {
    if (resetScreen) {
        currentInput = '';
        resetScreen = false;
    }
    
    if (currentInput === '0' && number !== '.') {
        currentInput = number;
    } else {
        // 防止输入多个小数点
        if (number === '.' && currentInput.includes('.')) {
            return;
        }
        currentInput += number;
    }
    
    updateDisplay();
}

// 选择操作符
function selectOperation(op) {
    if (op === 'neg') {  // 正负号切换
        currentInput = String(parseFloat(currentInput) * -1);
        updateDisplay();
        return;
    }
    
    if (op === 'backspace') {  // 退格
        if (currentInput.length > 1) {
            currentInput = currentInput.slice(0, -1);
        } else {
            currentInput = '0';
        }
        updateDisplay();
        return;
    }
    
    if (currentInput === '') return;
    
    if (previousInput !== '') {
        calculate();
    }
    
    operation = op;
    previousInput = currentInput;
    resetScreen = true;
}

// 计算结果
function calculate() {
    if (operation === null || resetScreen) {
        return;
    }
    
    let computation;
    const prev = parseFloat(previousInput);
    const current = parseFloat(currentInput);
    
    if (isNaN(prev) || isNaN(current)) {
        currentInput = 'Error';
        updateDisplay();
        clear();
        return;
    }
    
    switch (operation) {
        case '+':
            computation = prev + current;
            break;
        case '-':
            computation = prev - current;
            break;
        case '*':
            computation = prev * current;
            break;
        case '/':
            if (current === 0) {
                currentInput = 'Error';
                updateDisplay();
                clear();
                return;
            }
            computation = prev / current;
            break;
        default:
            return;
    }
    
    // 格式化结果，处理浮点数精度问题
    computation = parseFloat(computation.toFixed(10));
    currentInput = computation.toString();
    operation = null;
    previousInput = '';
    resetScreen = true;
    updateDisplay();
}

// 清除所有
function clear() {
    currentInput = '0';
    previousInput = '';
    operation = null;
    resetScreen = false;
    updateDisplay();
}

// 按钮点击事件
document.querySelectorAll('.btn').forEach(button => {
    button.addEventListener('click', () => {
        const value = button.getAttribute('data-value');
        
        if (['0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.'].includes(value)) {
            inputNumber(value);
        } else if (value === '=') {
            calculate();
        } else if (value === 'C') {
            clear();
        } else {
            selectOperation(value);
        }
    });
});

// 键盘支持
document.addEventListener('keydown', event => {
    if (event.key >= '0' && event.key <= '9') {
        inputNumber(event.key);
    } else if (event.key === '.') {
        inputNumber('.');
    } else if (['+', '-', '*', '/'].includes(event.key)) {
        selectOperation(event.key);
    } else if (event.key === 'Enter' || event.key === '=') {
        calculate();
    } else if (event.key === 'Escape') {
        clear();
    } else if (event.key === 'Backspace') {
        selectOperation('backspace');
    } else if (event.key === '%') {
        // 百分比功能
        currentInput = (parseFloat(currentInput) / 100).toString();
        updateDisplay();
    }
});

// 以下函数用于测试
function calculateTest(num1, operator, num2) {
    const prevCurrentInput = currentInput;
    const prevPreviousInput = previousInput;
    const prevOperation = operation;
    const prevResetScreen = resetScreen;
    
    currentInput = num2;
    previousInput = num1;
    operation = operator;
    
    let result;
    const n1 = parseFloat(num1);
    const n2 = parseFloat(num2);
    
    if (isNaN(n1) || isNaN(n2)) {
        result = 'Error';
    } else {
        switch (operator) {
            case '+':
                result = (n1 + n2).toString();
                break;
            case '-':
                result = (n1 - n2).toString();
                break;
            case '*':
                result = (n1 * n2).toString();
                break;
            case '/':
                if (n2 === 0) {
                    result = 'Error';
                } else {
                    result = (n1 / n2).toString();
                }
                break;
            default:
                result = 'Error';
        }
    }
    
    // 恢复状态
    currentInput = prevCurrentInput;
    previousInput = prevPreviousInput;
    operation = prevOperation;
    resetScreen = prevResetScreen;
    
    return result;
}

function calculateSequence(operations) {
    // 实现连续计算逻辑
    let result = parseFloat(operations[0]);
    
    for (let i = 1; i < operations.length; i += 2) {
        const operator = operations[i];
        const nextNum = parseFloat(operations[i + 1]);
        
        if (isNaN(result) || isNaN(nextNum)) {
            return 'Error';
        }
        
        switch (operator) {
            case '+':
                result += nextNum;
                break;
            case '-':
                result -= nextNum;
                break;
            case '*':
                result *= nextNum;
                break;
            case '/':
                if (nextNum === 0) {
                    return 'Error';
                }
                result /= nextNum;
                break;
            default:
                return 'Error';
        }
    }
    
    return result.toString();
}

function setInput(value) {
    currentInput = value;
    updateDisplay();
}

function getDisplayValue() {
    return display.textContent;
}

function clearCalculator() {
    clear();
}

function simulateKeyPress(key) {
    if (key >= '0' && key <= '9') {
        inputNumber(key);
    } else if (key === '.') {
        inputNumber('.');
    } else if (['+', '-', '*', '/'].includes(key)) {
        selectOperation(key);
    } else if (key === '=') {
        calculate();
    } else if (key === 'C' || key === 'Escape') {
        clear();
    }
}

// 导出函数供测试使用
window.calculate = calculateTest;
window.calculateSequence = calculateSequence;
window.setInput = setInput;
window.getDisplayValue = getDisplayValue;
window.clearCalculator = clearCalculator;
window.simulateKeyPress = simulateKeyPress;
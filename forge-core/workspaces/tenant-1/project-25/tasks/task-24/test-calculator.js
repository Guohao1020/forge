// Calculator functionality tests

// Test suite for basic operations
function testBasicAddition() {
  // Given calculator with two numbers
  const result = calculate('5', '+', '3');
  // Then result should be 8
  assert.equal(result, '8');
  console.log('✓ Basic addition test passed');
}

function testBasicSubtraction() {
  // Given calculator with two numbers
  const result = calculate('10', '-', '4');
  // Then result should be 6
  assert.equal(result, '6');
  console.log('✓ Basic subtraction test passed');
}

function testBasicMultiplication() {
  // Given calculator with two numbers
  const result = calculate('6', '*', '7');
  // Then result should be 42
  assert.equal(result, '42');
  console.log('✓ Basic multiplication test passed');
}

function testBasicDivision() {
  // Given calculator with two numbers
  const result = calculate('15', '/', '3');
  // Then result should be 5
  assert.equal(result, '5');
  console.log('✓ Basic division test passed');
}

function testDecimalNumbers() {
  // Given calculator with decimal numbers
  const result = calculate('3.5', '+', '2.5');
  // Then result should be 6.0
  assert.equal(result, '6.0');
  console.log('✓ Decimal numbers test passed');
}

function testDivisionByZero() {
  // Given division by zero
  const result = calculate('10', '/', '0');
  // Then should return error
  assert.equal(result, 'Error');
  console.log('✓ Division by zero test passed');
}

function testClearFunction() {
  // Given calculator with input
  setInput('123');
  clearCalculator();
  // Then display should be empty
  assert.equal(getDisplayValue(), '0');
  console.log('✓ Clear function test passed');
}

function testContinuousOperations() {
  // Given sequence of operations: 5 + 3 * 2
  const result = calculateSequence(['5', '+', '3', '*', '2']);
  // Then result should be 16 (following order of operations)
  assert.equal(result, '16');
  console.log('✓ Continuous operations test passed');
}

function testNegativeNumbers() {
  // Given negative number operation
  const result = calculate('-5', '+', '3');
  // Then result should be -2
  assert.equal(result, '-2');
  console.log('✓ Negative numbers test passed');
}

function testMultipleDecimals() {
  // Given calculation with multiple decimals
  const result = calculate('0.1', '+', '0.2');
  // Then result should be 0.3 (or close due to floating point precision)
  assert.ok(Math.abs(parseFloat(result) - 0.3) < 0.0001);
  console.log('✓ Multiple decimals test passed');
}

function testLargeNumbers() {
  // Given very large numbers
  const result = calculate('999999999', '*', '999999999');
  // Then result should handle large number correctly
  assert.ok(!isNaN(result) && result !== 'Error');
  console.log('✓ Large numbers test passed');
}

function testInvalidInput() {
  // Given invalid input
  const result = calculate('abc', '+', 'def');
  // Then should return error
  assert.equal(result, 'Error');
  console.log('✓ Invalid input test passed');
}

// Keyboard support tests
function testKeyboardInput() {
  // Given keyboard key press for number
  simulateKeyPress('5');
  // Then calculator display should show 5
  assert.equal(getDisplayValue(), '5');
  console.log('✓ Keyboard input test passed');
}

function testKeyboardOperation() {
  // Given keyboard key press for operation
  setInput('10');
  simulateKeyPress('+');
  setInput('5');
  simulateKeyPress('=');
  // Then result should be 15
  assert.equal(getDisplayValue(), '15');
  console.log('✓ Keyboard operation test passed');
}

// Error handling tests
function testMultipleOperators() {
  // Given input with multiple consecutive operators
  const result = calculate('5', '+', '*', '3');
  // Then should handle gracefully
  assert.ok(result === 'Error' || isValidNumber(result));
  console.log('✓ Multiple operators test passed');
}

// Helper function to check if value is a valid number
function isValidNumber(value) {
  return !isNaN(parseFloat(value)) && isFinite(parseFloat(value));
}

// Helper functions for testing
function calculate(num1, operator, num2) {
  // This will be implemented in script.js
  return window.calculate ? window.calculate(num1, operator, num2) : 'Error';
}

function calculateSequence(operations) {
  // This will be implemented in script.js
  return window.calculateSequence ? window.calculateSequence(operations) : 'Error';
}

function setInput(value) {
  // This will be implemented in script.js
  if (window.setInput) window.setInput(value);
}

function getDisplayValue() {
  // This will be implemented in script.js
  return window.getDisplayValue ? window.getDisplayValue() : '0';
}

function clearCalculator() {
  // This will be implemented in script.js
  if (window.clearCalculator) window.clearCalculator();
}

function simulateKeyPress(key) {
  // This will be implemented in script.js
  if (window.simulateKeyPress) window.simulateKeyPress(key);
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message || "Assertion failed");
  }
}

// Run all tests
try {
  testBasicAddition();
  testBasicSubtraction();
  testBasicMultiplication();
  testBasicDivision();
  testDecimalNumbers();
  testDivisionByZero();
  testClearFunction();
  testContinuousOperations();
  testNegativeNumbers();
  testMultipleDecimals();
  testLargeNumbers();
  testInvalidInput();
  testKeyboardInput();
  testKeyboardOperation();
  testMultipleOperators();
  
  console.log('\n✅ All tests passed successfully!');
} catch (error) {
  console.error('❌ Test failed:', error.message);
}
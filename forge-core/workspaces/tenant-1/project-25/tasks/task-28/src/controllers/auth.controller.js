import User from '../models/User.js';
import bcrypt from 'bcryptjs';

const saltRounds = 12;

export const authController = {
  // Register new user
  async register(req, res) {
    try {
      const { username, password, role } = req.body;
      
      // Check if user already exists
      const existingUser = await User.findOne({ username });
      if (existingUser) {
        return res.status(400).json({
          success: false,
          message: 'Username already exists'
        });
      }
      
      // Hash password before saving
      const hashedPassword = await bcrypt.hash(password, saltRounds);
      
      // Create new user
      const user = new User({
        username,
        password: hashedPassword,
        role
      });
      
      const savedUser = await user.save();
      
      res.status(201).json({
        success: true,
        message: 'User registered successfully',
        data: {
          id: savedUser._id,
          username: savedUser.username,
          role: savedUser.role
        }
      });
    } catch (error) {
      // Proper error handling without console.log
      console.error('Registration error:', error.message);
      res.status(500).json({
        success: false,
        message: 'Internal server error during registration'
      });
    }
  },

  // Login user
  async login(req, res) {
    try {
      const { username, password } = req.body;
      
      // Find user by username
      const user = await User.findOne({ username });
      if (!user) {
        return res.status(401).json({
          success: false,
          message: 'Invalid credentials'
        });
      }
      
      // Compare password using async bcrypt.compare
      const isValidPassword = await bcrypt.compare(password, user.password);
      if (!isValidPassword) {
        return res.status(401).json({
          success: false,
          message: 'Invalid credentials'
        });
      }
      
      res.json({
        success: true,
        message: 'Login successful',
        data: {
          id: user._id,
          username: user.username,
          role: user.role
        }
      });
    } catch (error) {
      // Proper error handling without console.log
      console.error('Login error:', error.message);
      res.status(500).json({
        success: false,
        message: 'Internal server error during login'
      });
    }
  },

  // Get user by ID
  async getUserById(req, res) {
    try {
      const { id } = req.params;
      const user = await User.findById(id);
      
      if (!user) {
        return res.status(404).json({
          success: false,
          message: 'User not found'
        });
      }
      
      res.json({
        success: true,
        data: user
      });
    } catch (error) {
      console.error('Get user error:', error.message);
      res.status(500).json({
        success: false,
        message: 'Internal server error'
      });
    }
  },

  // Update user profile
  async updateUserProfile(req, res) {
    try {
      const { id } = req.params;
      const updateData = req.body;
      
      // Don't allow updating password through this endpoint
      delete updateData.password;
      
      const updatedUser = await User.findByIdAndUpdate(
        id,
        updateData,
        { new: true, runValidators: true }
      );
      
      if (!updatedUser) {
        return res.status(404).json({
          success: false,
          message: 'User not found'
        });
      }
      
      res.json({
        success: true,
        message: 'Profile updated successfully',
        data: updatedUser
      });
    } catch (error) {
      console.error('Update user error:', error.message);
      res.status(500).json({
        success: false,
        message: 'Internal server error'
      });
    }
  }
};